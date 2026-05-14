package engine_test

import (
	"context"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/engine"
)

type testStorageNode struct {
	id      string
	address string
}

func (t *testStorageNode) ID() string      { return t.id }
func (t *testStorageNode) Address() string { return t.address }
func (*testStorageNode) WriteShard(_ context.Context, _, _ string, _ int, _ []byte) error {
	return nil
}
func (*testStorageNode) ReadShard(_ context.Context, _, _ string, _ int) ([]byte, error) {
	return nil, nil
}
func (*testStorageNode) ShardExists(_ context.Context, _, _ string, _ int) bool {
	return false
}

func TestRoundRobinPlacementAcrossRegisteredNodes(t *testing.T) {
	e := newTestEngine(t)

	if err := e.RegisterStorageNode(&testStorageNode{id: "node-a", address: "127.0.0.1:7001"}); err != nil {
		t.Fatalf("register node-a: %v", err)
	}
	if err := e.RegisterStorageNode(&testStorageNode{id: "node-b", address: "127.0.0.1:7002"}); err != nil {
		t.Fatalf("register node-b: %v", err)
	}

	placements, err := e.PlanShardPlacement(context.Background(), engine.PlacementRequest{
		Bucket:     "bucket",
		Key:        "key",
		Hash:       "hash",
		ShardCount: 6,
	})
	if err != nil {
		t.Fatalf("plan shard placement: %v", err)
	}
	if len(placements) != 6 {
		t.Fatalf("placements len = %d, want %d", len(placements), 6)
	}

	expected := []string{engine.DefaultLocalNodeID, "node-a", "node-b", engine.DefaultLocalNodeID, "node-a", "node-b"}
	for index, placement := range placements {
		if placement.Index != index {
			t.Errorf("placement index = %d, want %d", placement.Index, index)
		}
		if placement.NodeID != expected[index] {
			t.Errorf("placement[%d].node_id = %q, want %q", index, placement.NodeID, expected[index])
		}
	}
}

func TestRegisterStorageNodeRejectsEmptyID(t *testing.T) {
	e := newTestEngine(t)
	if err := e.RegisterStorageNode(&testStorageNode{id: " ", address: "127.0.0.1:7003"}); err == nil {
		t.Fatal("expected register with empty id to fail")
	}
}
