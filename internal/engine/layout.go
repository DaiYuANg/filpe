package engine

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Layout maps object keys to their shard locations.
type Layout struct {
	ShardDir  string
	Hash      string
	Shards    []Shard
	Bucket    string
	Key       string
	Size      int64
	ETag      string
	ShardSize int64
	CoderType string
	Version   int64
}

func (l Layout) String() string {
	return fmt.Sprintf("layout{%s:%s,shards=%d,hash=%s}", l.Bucket, l.Key, len(l.Shards), l.Hash)
}

func layoutKey(bucket string, key string) string {
	return bucket + "/" + key
}

func layoutHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:16]) // use first 16 bytes as hash
}

func makeShardDir(prefix string) string {
	return strings.ToLower(prefix[:2])
}
