package engine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
)

// Default config (MinIO-style 9+3)
const (
	DefaultDataChunks   = 9
	DefaultParityChunks = 3
	DefaultShardSize    = int64(1 << 20) // 1MB shard size
)

// Engine is the erasure-coded object storage engine.
type Engine struct {
	mu          sync.RWMutex
	fs          afero.Fs
	root        string
	dataChunks  int
	parityChunks int
	shardSize   int64
	coder       *Coder
	backend     *shardBackend
	layoutCache sync.Map // string -> *Layout
}

// ObjectMeta stores object metadata (moved from metadata module).
type ObjectMeta struct {
	Bucket      string    `json:"bucket"`
	Key         string    `json:"key"`
	Hash        string    `json:"hash"`
	ETag        string    `json:"etag"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ObjectInfo is ObjectMeta + erasure coding info.
type ObjectInfo struct {
	ObjectMeta
	DataChunks   int `json:"data_chunks"`
	ParityChunks int `json:"parity_chunks"`
	TotalChunks  int `json:"total_chunks"`
	ShardSize    int64 `json:"shard_size"`
}

// Health reports the health of a shard set.
type Health struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	TotalChunks int    `json:"total_chunks"`
	Available   int    `json:"available"`
	Missing     int    `json:"missing"`
	Recoverable bool   `json:"recoverable"`
}

func (h Health) String() string {
	return fmt.Sprintf("health{%s:%s,%d/%d available, recoverable=%v}", h.Bucket, h.Key, h.Available, h.TotalChunks, h.Recoverable)
}

// ErrObjectNotFound is returned when an object does not exist.
var ErrObjectNotFound = errors.New("engine: object not found")

// ErrShardRecoveryFailed is returned when shard recovery fails.
var ErrShardRecoveryFailed = errors.New("engine: shard recovery failed")

func NewEngine(dataDir string, dataChunks, parityChunks int, fs afero.Fs) (*Engine, error) {
	if dataChunks < 1 {
		dataChunks = DefaultDataChunks
	}
	if parityChunks < 1 {
		parityChunks = DefaultParityChunks
	}
	if dataDir == "" {
		dataDir = "./data/engine"
	}
	if fs == nil {
		fs = afero.NewOsFs()
	}
	root := dataDir
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	
	shardSize := int64(DefaultShardSize)
	backend := newShardBackend(fs, root, shardSize)
	coder, err := newCoder(CoderConfig{
		DataChunks:   dataChunks,
		ParityChunks: parityChunks,
		ShardPool:    backend,
	})
	if err != nil {
		return nil, err
	}
	
	return &Engine{
		fs:           fs,
		root:         root,
		dataChunks:   dataChunks,
		parityChunks: parityChunks,
		shardSize:    shardSize,
		coder:        coder,
		backend:      backend,
	}, nil
}

// PutObject writes an object to the engine with erasure coding.
func (e *Engine) PutObject(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (ObjectInfo, error) {
	bucket = strings.TrimSpace(bucket)
	key = strings.TrimSpace(key)
	if bucket == "" || key == "" {
		return ObjectInfo{}, errors.New("engine: bucket and key are required")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("engine: read input: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	etag := `"` + hash + `"`

	// Encode object into shards
	shards, err := e.coder.Encode(data)
	if err != nil {
		return ObjectInfo{}, err
	}

	shardDir := makeShardDir(key)
	// Write all shards to disk
	for i, shard := range shards {
		if err := e.backend.WriteShard(shardDir, hash, i, shard); err != nil {
			return ObjectInfo{}, fmt.Errorf("engine: write shard %d: %w", i, err)
		}
	}

	// Write metadata
	metaBytes, err := json.Marshal(&Layout{
		ShardDir:  shardDir,
		Hash:      hash,
		Bucket:    bucket,
		Key:       key,
		Size:      int64(len(data)),
		ETag:      etag,
		ShardSize: e.shardSize,
		CoderType: fmt.Sprintf("%d+%d", e.dataChunks, e.parityChunks),
		Version:   1,
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("engine: marshal layout: %w", err)
	}
	if err := e.backend.WriteMeta(shardDir, hash, metaBytes); err != nil {
		return ObjectInfo{}, fmt.Errorf("engine: write meta: %w", err)
	}

	// Update cache
	e.layoutCache.Store(layoutKey(bucket, key), &Layout{
		ShardDir:  shardDir,
		Hash:      hash,
		Bucket:    bucket,
		Key:       key,
		Size:      int64(len(data)),
		ETag:      etag,
		ShardSize: e.shardSize,
		CoderType: fmt.Sprintf("%d+%d", e.dataChunks, e.parityChunks),
		Version:   1,
	})

	return ObjectInfo{
		ObjectMeta: ObjectMeta{
			Bucket:      bucket,
			Key:         key,
			Hash:        hash,
			ETag:        etag,
			Size:        int64(len(data)),
			ContentType: contentType,
			UpdatedAt:   time.Now().UTC(),
		},
		DataChunks:   e.dataChunks,
		ParityChunks: e.parityChunks,
		TotalChunks:  e.dataChunks + e.parityChunks,
		ShardSize:    e.shardSize,
	}, nil
}

// GetObject reads and reconstructs an object from shards.
func (e *Engine) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, ObjectInfo, error) {
	layout, err := e.getLayout(bucket, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}

	// Read available shards
	total := e.coder.TotalChunks()
	shards := make([][]byte, total)
	available := 0
	for i := 0; i < total; i++ {
		data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
		if err != nil {
			return nil, ObjectInfo{}, fmt.Errorf("engine: read shard %d: %w", i, err)
		}
		if data != nil {
			shards[i] = data
			available++
		}
	}

	// Check if we have enough shards to decode
	if available < e.dataChunks {
		// Try to rebuild missing shards first
		if e.canRebuild(layout) {
			if err := e.rebuildShards(layout); err != nil {
				return nil, ObjectInfo{}, ErrShardRecoveryFailed
			}
			// Read again after rebuild
			for i := 0; i < total; i++ {
				if shards[i] == nil {
					data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
					if err != nil {
						return nil, ObjectInfo{}, fmt.Errorf("engine: re-read shard %d: %w", i, err)
					}
					shards[i] = data
					available++
				}
			}
		}
		if available < e.dataChunks {
			return nil, ObjectInfo{}, ErrShardRecoveryFailed
		}
	}

	// Decode data
	decoded, err := e.coder.Decode(shards)
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("engine: decode: %w", err)
	}

	return io.NopCloser(strings.NewReader(string(decoded))), ObjectInfo{
		ObjectMeta: ObjectMeta{
			Bucket:      layout.Bucket,
			Key:         layout.Key,
			Hash:        layout.Hash,
			ETag:        layout.ETag,
			Size:        layout.Size,
			ContentType: "",
			UpdatedAt:   time.Now().UTC(),
		},
		DataChunks:   e.dataChunks,
		ParityChunks: e.parityChunks,
		TotalChunks:  e.dataChunks + e.parityChunks,
		ShardSize:    e.shardSize,
	}, nil
}

// DeleteObject removes an object and its shards.
func (e *Engine) DeleteObject(ctx context.Context, bucket, key string) error {
	layout, err := e.getLayout(bucket, key)
	if err != nil {
		return err
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	e.layoutCache.Delete(layoutKey(bucket, key))
	return e.backend.DeleteShardSet(layout.ShardDir, layout.Hash)
}

// CheckHealth checks the health of a shard set.
func (e *Engine) CheckHealth(ctx context.Context, bucket, key string) (Health, error) {
	layout, err := e.getLayout(bucket, key)
	if err != nil {
		return Health{}, err
	}

	total := e.coder.TotalChunks()
	available := 0
	for i := 0; i < total; i++ {
		if e.backend.ShardExists(layout.ShardDir, layout.Hash, i) {
			available++
		}
	}

	return Health{
		Bucket:      layout.Bucket,
		Key:         layout.Key,
		TotalChunks: total,
		Available:   available,
		Missing:     total - available,
		Recoverable: available >= e.dataChunks,
	}, nil
}

// ListObjects returns metadata for all objects in the engine (scans filesystem).
func (e *Engine) ListObjects(ctx context.Context, bucket string) ([]ObjectInfo, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, errors.New("engine: bucket is required")
	}

	var results []ObjectInfo
	// Walk the root directory to find all shard sets
    walkFn := func(path string, info os.FileInfo, err error) error {
    if err != nil {
        return err
    }
		if info.IsDir() {
			return nil
		}
		if info.Name() == "meta.json" {
			data, err := afero.ReadFile(e.fs, path)
			if err != nil {
				return nil
			}
			var layout Layout
			if err := json.Unmarshal(data, &layout); err != nil {
				return nil
			}
			if layout.Bucket == bucket {
				info := ObjectInfo{
					ObjectMeta: ObjectMeta{
						Bucket:      layout.Bucket,
						Key:         layout.Key,
						Hash:        layout.Hash,
						ETag:        layout.ETag,
						Size:        layout.Size,
						ContentType: "",
						UpdatedAt:   time.Now().UTC(),
					},
					DataChunks:   e.dataChunks,
					ParityChunks: e.parityChunks,
					TotalChunks:  e.dataChunks + e.parityChunks,
					ShardSize:    e.shardSize,
				}
				results = append(results, info)
			}
		}
		return nil
	}
	_ = afero.Walk(e.fs, e.root, walkFn)
	return results, nil
}

// getLayout retrieves layout from cache or disk.
func (e *Engine) getLayout(bucket, key string) (*Layout, error) {
	lk := layoutKey(bucket, key)
	
	// Try cache first
	if cached, ok := e.layoutCache.Load(lk); ok {
		return cached.(*Layout), nil
	}

	// Compute shard dir and hash
	shardDir := makeShardDir(key)
	hash := layoutHash(lk)

	// Try to read from disk
	metaBytes, err := e.backend.ReadMeta(shardDir, hash)
	if err != nil {
		return nil, ErrObjectNotFound
	}

	var layout Layout
	if err := json.Unmarshal(metaBytes, &layout); err != nil {
		return nil, fmt.Errorf("engine: unmarshal layout: %w", err)
	}

	// Cache it
	e.layoutCache.Store(lk, &layout)
	return &layout, nil
}

func (e *Engine) canRebuild(layout *Layout) bool {
	total := e.coder.TotalChunks()
	available := 0
	for i := 0; i < total; i++ {
		if e.backend.ShardExists(layout.ShardDir, layout.Hash, i) {
			available++
		}
	}
	return available >= e.dataChunks
}

func (e *Engine) rebuildShards(layout *Layout) error {
	total := e.coder.TotalChunks()
	shards := make([][]byte, total)
	
	for i := 0; i < total; i++ {
		data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
		if err != nil {
			return fmt.Errorf("engine: read shard for rebuild: %w", err)
		}
		shards[i] = data
	}

	// Use coder to rebuild missing parity shards
	// We only need to rebuild parity shards, data shards are already available
	if err := e.coder.Rebuild(shards); err != nil {
		return err
	}

	// Write rebuilt shards
	for i := 0; i < total; i++ {
		if shards[i] != nil {
			if err := e.backend.WriteShard(layout.ShardDir, layout.Hash, i, shards[i]); err != nil {
				return fmt.Errorf("engine: write rebuilt shard: %w", err)
			}
		}
	}
	return nil
}

