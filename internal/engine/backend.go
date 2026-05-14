package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Shard represents a single erasure-coded shard.
type Shard struct {
	Index    int
	Size     int64
	Data     []byte
	Verified bool
}

func (s Shard) String() string {
	return fmt.Sprintf("shard{%d,%d}", s.Index, s.Size)
}

// shardBackend manages shard lifecycle with a backing filesystem.
type shardBackend struct {
	fs        afero.Fs
	root      string
	shardSize int64
}

func newShardBackend(fs afero.Fs, root string, shardSize int64) *shardBackend {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	if root == "" {
		root = "./data/engine"
	}
	_ = fs.MkdirAll(root, 0755)
	return &shardBackend{fs: fs, root: root, shardSize: shardSize}
}

func (b *shardBackend) ShardSize() int64 { return b.shardSize }

// shardPath returns the filesystem path for a given shard.
func (b *shardBackend) shardPath(shardDir string, hash string, index int) string {
	return filepath.Join(b.root, shardDir, hash, fmt.Sprintf("chunk-%04d", index))
}

func (b *shardBackend) metaPath(shardDir string, hash string) string {
	return filepath.Join(b.root, shardDir, hash, "meta.json")
}

// WriteShard writes a shard to disk.
func (b *shardBackend) WriteShard(shardDir, hash string, index int, data []byte) error {
	path := b.shardPath(shardDir, hash, index)
	if err := b.fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("engine: create shard dir: %w", err)
	}
	file, err := b.fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("engine: open shard file: %w", err)
	}
	_, err = file.Write(data)
	if cerr := file.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

// WriteMeta writes the shard set metadata to disk.
func (b *shardBackend) WriteMeta(shardDir, hash string, data []byte) error {
	path := b.metaPath(shardDir, hash)
	if err := b.fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("engine: create meta dir: %w", err)
	}
	file, err := b.fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("engine: open meta file: %w", err)
	}
	_, err = file.Write(data)
	if cerr := file.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

// ReadShard reads a shard from disk, returning nil if the shard is missing.
func (b *shardBackend) ReadShard(shardDir, hash string, index int) ([]byte, error) {
	path := b.shardPath(shardDir, hash, index)
	file, err := b.fs.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("engine: read shard %s-%d: %w", hash, index, err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	return data, err
}

// ShardExists checks if a shard exists on disk.
func (b *shardBackend) ShardExists(shardDir, hash string, index int) bool {
	path := b.shardPath(shardDir, hash, index)
	_, err := b.fs.Stat(path)
	return err == nil
}

// ReadMeta reads shard set metadata from disk.
func (b *shardBackend) ReadMeta(shardDir, hash string) ([]byte, error) {
	path := b.metaPath(shardDir, hash)
	data, err := afero.ReadFile(b.fs, path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DeleteShardSet removes an entire shard set from disk.
func (b *shardBackend) DeleteShardSet(shardDir, hash string) error {
	dir := filepath.Join(b.root, shardDir, hash)
	return b.fs.RemoveAll(dir)
}

// ListShards returns the shard indexes that exist for a shard set.
func (b *shardBackend) ListShards(shardDir, hash string) ([]int, error) {
	dir := filepath.Join(b.root, shardDir, hash)
	entries, err := afero.ReadDir(b.fs, dir)
	if err != nil {
		return nil, err
	}
	var indexes []int
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "chunk-") {
			var idx int
			fmt.Sscanf(entry.Name(), "chunk-%d", &idx)
			indexes = append(indexes, idx)
		}
	}
	return indexes, nil
}
