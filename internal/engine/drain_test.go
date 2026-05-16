package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/engine"
	"github.com/lyonbrown4d/maxio/internal/model"
)

func TestDrainStorageNodeExcludesNewPlacements(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(t)
	nodeA := newInMemoryStorageNode("node-a", "127.0.0.1:7001")
	nodeB := newInMemoryStorageNode("node-b", "127.0.0.1:7002")
	if err := registerDistributedPlacementNodes(t, eng, nodeA, nodeB); err != nil {
		t.Fatal(err)
	}

	if err := eng.DrainStorageNode(nodeA.id); err != nil {
		t.Fatalf("drain storage node: %v", err)
	}

	meta := putObjectForDrain(ctx, t, eng, "drain-key.txt")
	assertPlacementExcludesNode(t, meta.ShardPlacements, nodeA.id)
	assertPlacementIncludesNode(t, meta.ShardPlacements, engine.DefaultLocalNodeID)
	assertPlacementIncludesNode(t, meta.ShardPlacements, nodeB.id)
}

func TestResumeStorageNodeRestoresNewPlacements(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(t)
	nodeA := newInMemoryStorageNode("node-a", "127.0.0.1:7001")
	nodeB := newInMemoryStorageNode("node-b", "127.0.0.1:7002")
	if err := registerDistributedPlacementNodes(t, eng, nodeA, nodeB); err != nil {
		t.Fatal(err)
	}
	if err := eng.DrainStorageNode(nodeA.id); err != nil {
		t.Fatalf("drain storage node: %v", err)
	}
	if err := eng.ResumeStorageNode(nodeA.id); err != nil {
		t.Fatalf("resume storage node: %v", err)
	}

	meta := putObjectForDrain(ctx, t, eng, "resume-key.txt")
	assertPlacementIncludesNode(t, meta.ShardPlacements, nodeA.id)
}

func putObjectForDrain(ctx context.Context, t *testing.T, eng *engine.Engine, key string) engine.ObjectInfo {
	t.Helper()
	meta, err := eng.PutObject(ctx, "test-bucket", key, strings.NewReader("drain placement payload"), "text/plain")
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	return meta
}

func assertPlacementExcludesNode(t *testing.T, placements []model.ShardPlacement, nodeID string) {
	t.Helper()
	for _, placement := range placements {
		if placement.NodeID == nodeID {
			t.Fatalf("placement includes drained node %q", nodeID)
		}
	}
}

func assertPlacementIncludesNode(t *testing.T, placements []model.ShardPlacement, nodeID string) {
	t.Helper()
	for _, placement := range placements {
		if placement.NodeID == nodeID {
			return
		}
	}
	t.Fatalf("placement does not include node %q", nodeID)
}
