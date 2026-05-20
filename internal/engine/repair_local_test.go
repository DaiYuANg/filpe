package engine_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/engine"
)

func TestRepairObjectRestoresMissingLocalShard(t *testing.T) {
	ctx := context.Background()
	e := newTestEngine(t)

	content := []byte("missing local shard should be rebuilt by repair")
	meta, err := e.PutObject(ctx, "test-bucket", "missing-shard.txt", bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	const missingIndex = 0
	deleteMissingLocalShard(ctx, t, e, meta, missingIndex)
	assertMissingShardHealth(ctx, t, e)

	result, err := e.RepairObject(ctx, "test-bucket", "missing-shard.txt")
	if err != nil {
		t.Fatalf("RepairObject: %v", err)
	}
	assertMissingShardRepairResult(ctx, t, e, meta, result, missingIndex)
	assertMissingShardObjectReadable(ctx, t, e, content)
}

func deleteMissingLocalShard(
	ctx context.Context,
	t *testing.T,
	e *engine.Engine,
	meta engine.ObjectInfo,
	missingIndex int,
) {
	t.Helper()
	deleteErr := e.DeleteLocalShard(ctx, meta.ShardDir, meta.Hash, missingIndex)
	if deleteErr != nil {
		t.Fatalf("delete local shard: %v", deleteErr)
	}
}

func assertMissingShardHealth(ctx context.Context, t *testing.T, e *engine.Engine) {
	t.Helper()
	health, err := e.CheckHealth(ctx, "test-bucket", "missing-shard.txt")
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if health.Missing != 1 {
		t.Fatalf("Missing = %d, want 1", health.Missing)
	}
	if !health.Recoverable {
		t.Fatal("Recoverable should be true with one missing shard")
	}
}

func assertMissingShardRepairResult(
	ctx context.Context,
	t *testing.T,
	e *engine.Engine,
	meta engine.ObjectInfo,
	result engine.RepairResult,
	missingIndex int,
) {
	t.Helper()
	if !intSliceContains(result.Repaired, missingIndex) {
		t.Fatalf("repaired shards = %v, want %d", result.Repaired, missingIndex)
	}
	if result.HealthAfter.Missing != 0 || result.HealthAfter.Corrupted != 0 {
		t.Fatalf("HealthAfter = %+v, want no missing or corrupted shards", result.HealthAfter)
	}
	if !e.LocalShardExists(ctx, meta.ShardDir, meta.Hash, missingIndex) {
		t.Fatal("repaired shard does not exist")
	}
}

func assertMissingShardObjectReadable(ctx context.Context, t *testing.T, e *engine.Engine, content []byte) {
	t.Helper()
	reader, _, err := e.GetObject(ctx, "test-bucket", "missing-shard.txt")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Fatalf("close reader: %v", closeErr)
		}
	}()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read object data: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("data = %q, want %q", data, content)
	}
}

func intSliceContains(values []int, expected int) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
