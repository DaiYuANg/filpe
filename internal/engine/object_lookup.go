package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/afero"
)

func (e *Engine) ListObjects(ctx context.Context, bucket string) ([]ObjectInfo, error) {
	_ = ctx
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, errors.New("engine: bucket is required")
	}
	return e.scanObjectInfos(bucket)
}

func (e *Engine) scanObjectInfos(bucket string) ([]ObjectInfo, error) {
	var results []ObjectInfo
	walkFn := func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || info.Name() != "meta.json" {
			return nil
		}
		objectInfo, ok, err := e.readObjectInfo(path, bucket)
		if err != nil {
			return err
		}
		if ok {
			results = append(results, objectInfo)
		}
		return nil
	}
	if err := afero.Walk(e.fs, e.root, walkFn); err != nil {
		return nil, fmt.Errorf("engine: scan object layouts: %w", err)
	}
	return results, nil
}

func (e *Engine) readObjectInfo(path, bucket string) (ObjectInfo, bool, error) {
	data, err := afero.ReadFile(e.fs, path)
	if err != nil {
		return ObjectInfo{}, false, fmt.Errorf("engine: read layout %s: %w", path, err)
	}
	var layout Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		return ObjectInfo{}, false, fmt.Errorf("engine: unmarshal layout %s: %w", path, err)
	}
	if layout.Bucket != bucket {
		return ObjectInfo{}, false, nil
	}
	return e.objectInfoFromLayout(&layout), true, nil
}

// getLayout retrieves layout from cache or disk.
func (e *Engine) getLayout(bucket, key string) (*Layout, error) {
	lk := layoutKey(bucket, key)

	// Try cache first
	if cached, ok := e.layoutCache.Load(lk); ok {
		layout, ok := cached.(*Layout)
		if ok {
			return layout, nil
		}
		e.layoutCache.Delete(lk)
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
	for i := range total {
		if e.backend.ShardExists(layout.ShardDir, layout.Hash, i) {
			available++
		}
	}
	return available >= e.dataChunks
}

func (e *Engine) rebuildShards(layout *Layout) error {
	total := e.coder.TotalChunks()
	shards := make([][]byte, total)

	for i := range total {
		data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
		if err != nil {
			return fmt.Errorf("engine: read shard for rebuild: %w", err)
		}
		shards[i] = data
	}

	// Use coder to rebuild missing parity shards
	// We only need to rebuild parity shards, data shards are already available
	if err := e.coder.Rebuild(shards); err != nil {
		return fmt.Errorf("engine: rebuild shards: %w", err)
	}

	// Write rebuilt shards
	for i := range total {
		if shards[i] != nil {
			if err := e.backend.WriteShard(layout.ShardDir, layout.Hash, i, shards[i]); err != nil {
				return fmt.Errorf("engine: write rebuilt shard: %w", err)
			}
		}
	}
	return nil
}

func (e *Engine) readAvailableShards(layout *Layout) ([][]byte, int, error) {
	total := e.coder.TotalChunks()
	shards := make([][]byte, total)
	available := 0
	for i := range total {
		data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
		if err != nil {
			return nil, 0, fmt.Errorf("engine: read shard %d: %w", i, err)
		}
		if data == nil {
			continue
		}
		shards[i] = data
		available++
	}
	return shards, available, nil
}

func (e *Engine) ensureReadableShards(layout *Layout, shards [][]byte, available int) error {
	if available >= e.dataChunks {
		return nil
	}
	if !e.canRebuild(layout) {
		return ErrShardRecoveryFailed
	}
	if err := e.rebuildShards(layout); err != nil {
		return fmt.Errorf("%w: %w", ErrShardRecoveryFailed, err)
	}
	available, err := e.fillMissingShards(layout, shards)
	if err != nil {
		return err
	}
	if available < e.dataChunks {
		return ErrShardRecoveryFailed
	}
	return nil
}

func (e *Engine) fillMissingShards(layout *Layout, shards [][]byte) (int, error) {
	available := 0
	for i := range shards {
		if shards[i] != nil {
			available++
			continue
		}
		data, err := e.backend.ReadShard(layout.ShardDir, layout.Hash, i)
		if err != nil {
			return 0, fmt.Errorf("engine: re-read shard %d: %w", i, err)
		}
		if data == nil {
			continue
		}
		shards[i] = data
		available++
	}
	return available, nil
}

func (e *Engine) objectInfoFromLayout(layout *Layout) ObjectInfo {
	return ObjectInfo{
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
}
