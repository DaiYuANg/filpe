package metadata_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/metadata"
	"github.com/lyonbrown4d/maxio/internal/model"
)

func TestInMemoryMetadataBlobRefStoresShardPlacements(t *testing.T) {
	meta := metadata.NewInMemoryMetadata()

	placements := []model.ShardPlacement{
		{Index: 0, NodeID: "node-a", NodeAddress: "127.0.0.1:9001", Local: true},
		{Index: 1, NodeID: "node-b", NodeAddress: "127.0.0.1:9002", Local: true},
	}
	checksums := []string{"checksum-a", "checksum-b"}
	err := meta.CreateBlobRef(context.Background(), "hash", "shard-dir", 2048, placements, checksums)
	if err != nil {
		t.Fatalf("create blob ref: %v", err)
	}

	ref, ok, err := meta.GetBlobRef(context.Background(), "hash")
	if err != nil {
		t.Fatalf("get blob ref: %v", err)
	}
	if !ok {
		t.Fatal("blob ref not found")
	}
	if !reflect.DeepEqual(ref.ShardPlacements, placements) {
		t.Fatalf("stored shard placements %#v, want %#v", ref.ShardPlacements, placements)
	}
	if !reflect.DeepEqual(ref.ShardChecksums, checksums) {
		t.Fatalf("stored shard checksums %#v, want %#v", ref.ShardChecksums, checksums)
	}

	placements[0].NodeID = "changed"
	checksums[0] = "changed"
	if ref.ShardPlacements[0].NodeID != "node-a" {
		t.Fatalf("stored shard placements mutated by caller: %q", ref.ShardPlacements[0].NodeID)
	}
	if ref.ShardChecksums[0] != "checksum-a" {
		t.Fatalf("stored shard checksums mutated by caller: %q", ref.ShardChecksums[0])
	}
}
