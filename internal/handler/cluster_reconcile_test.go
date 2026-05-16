package handler_test

import (
	"testing"

	"github.com/lyonbrown4d/maxio/internal/discovery"
	"github.com/lyonbrown4d/maxio/internal/handler"
	raftx "github.com/lyonbrown4d/maxio/internal/raft"
)

func TestBuildClusterReconcilePlanAddsAliveDiscoveredNodes(t *testing.T) {
	t.Parallel()

	plan := handler.BuildClusterReconcilePlan(raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "127.0.0.1:63000",
		},
	}, []discovery.Node{
		{ReplicaID: 2, RaftAddress: "127.0.0.1:63001", State: "alive"},
		{ReplicaID: 3, RaftAddress: "127.0.0.1:63002", State: "suspect"},
	}, false)

	if plan.Mode != "add_only" {
		t.Fatalf("mode = %q, want add_only", plan.Mode)
	}
	if got := plan.Desired[2]; got != "127.0.0.1:63001" {
		t.Fatalf("desired replica 2 = %q", got)
	}
	if _, ok := plan.Desired[3]; ok {
		t.Fatal("suspect replica should not be added")
	}
	if len(plan.Added) != 1 || plan.Added[0].ReplicaID != 2 {
		t.Fatalf("added = %+v", plan.Added)
	}
}

func TestBuildClusterReconcilePlanKeepsLocalWhenExact(t *testing.T) {
	t.Parallel()

	plan := handler.BuildClusterReconcilePlan(raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "127.0.0.1:63000",
			2: "127.0.0.1:63001",
		},
	}, []discovery.Node{
		{ReplicaID: 3, RaftAddress: "127.0.0.1:63002", State: "alive"},
	}, true)

	if plan.Mode != "exact" {
		t.Fatalf("mode = %q, want exact", plan.Mode)
	}
	if _, ok := plan.Desired[1]; !ok {
		t.Fatal("local replica should be retained in exact mode")
	}
	if _, ok := plan.Desired[2]; ok {
		t.Fatal("missing non-local replica should be removed in exact mode")
	}
	if len(plan.Removed) != 1 || plan.Removed[0] != 2 {
		t.Fatalf("removed = %+v", plan.Removed)
	}
}

func TestBuildClusterReconcilePlanReportsTargetConflicts(t *testing.T) {
	t.Parallel()

	plan := handler.BuildClusterReconcilePlan(raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "127.0.0.1:63000",
			2: "127.0.0.1:63001",
		},
	}, []discovery.Node{
		{ReplicaID: 2, RaftAddress: "127.0.0.1:63999", State: "alive"},
	}, false)

	if len(plan.Conflicts) != 1 {
		t.Fatalf("conflicts = %+v", plan.Conflicts)
	}
	if plan.Desired[2] != "127.0.0.1:63001" {
		t.Fatalf("conflict changed desired target to %q", plan.Desired[2])
	}
}
