package store_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/lyonbrown4d/maxio/internal/engine"
	"github.com/lyonbrown4d/maxio/internal/metadata"
	"github.com/lyonbrown4d/maxio/internal/model"
	"github.com/lyonbrown4d/maxio/internal/store"
)

func TestStoreRecoverRemovesExpiredPendingObjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	meta := metadata.NewInMemoryMetadata()
	storeModule, err := store.NewStore(t.TempDir(), meta, nil)
	mustNoError(t, err, "new store")
	mustNoError(t, storeModule.CreateBucket(ctx, "bucket"), "create bucket")
	mustNoError(t, meta.StageObjectMeta(ctx, model.ObjectMeta{
		Bucket:    "bucket",
		Key:       "pending.txt",
		Hash:      "pending-hash",
		UpdatedAt: time.Now().UTC().Add(-2 * time.Hour),
	}), "stage pending object")

	result, err := storeModule.Recover(ctx, store.RecoveryOptions{PendingTTL: time.Hour})
	mustNoError(t, err, "recover store")
	if result.PendingRemoved != 1 {
		t.Fatalf("pending removed = %d, want 1", result.PendingRemoved)
	}
	staged, err := meta.ListStagedObjectMetas(ctx, "", "")
	mustNoError(t, err, "list staged objects")
	if len(staged) != 0 {
		t.Fatalf("staged objects = %+v, want empty", staged)
	}
}

func TestStoreRecoverRemovesOrphanShardSets(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	meta := metadata.NewInMemoryMetadata()
	eng, err := engine.NewEngine(t.TempDir(), engine.DefaultDataChunks, engine.DefaultParityChunks, nil)
	mustNoError(t, err, "new engine")
	blob, err := eng.PutBlob(ctx, "orphan.txt", bytes.NewReader([]byte("orphan payload")))
	mustNoError(t, err, "put orphan blob")
	storeModule, err := store.NewStore("", meta, eng)
	mustNoError(t, err, "new store")

	result, err := storeModule.Recover(ctx, store.RecoveryOptions{
		PendingTTL:          time.Hour,
		CleanupOrphanShards: true,
	})
	mustNoError(t, err, "recover store")
	if result.OrphanShardCleanup.Removed != 1 {
		t.Fatalf("orphan removed = %d, want 1", result.OrphanShardCleanup.Removed)
	}
	if eng.LocalShardExists(ctx, blob.ShardDir, blob.Hash, 0) {
		t.Fatal("orphan shard still exists")
	}
}
