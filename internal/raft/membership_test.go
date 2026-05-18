package raft

import (
	"testing"
)

func TestNormalizeDesiredReplicasIncludesLocalReplica(t *testing.T) {
	t.Parallel()

	desired, err := normalizeDesiredReplicas(map[uint64]string{
		1: "127.0.0.1:63000",
		2: "127.0.0.1:63001",
	}, 2)
	if err != nil {
		t.Fatalf("normalize desired replicas: %v", err)
	}
	if got := desired[1]; got != "127.0.0.1:63000" {
		t.Fatalf("desired[1] = %q", got)
	}
}

func TestNormalizeDesiredReplicasRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := normalizeDesiredReplicas(nil, 1)
	if err == nil {
		t.Fatal("expected empty desired replicas validation error")
	}
}

func TestNormalizeDesiredReplicasRejectsMissingLocalReplica(t *testing.T) {
	t.Parallel()

	_, err := normalizeDesiredReplicas(map[uint64]string{
		1: "127.0.0.1:63000",
	}, 2)
	if err == nil {
		t.Fatal("expected missing local replica validation error")
	}
}

func TestNormalizeDesiredReplicasRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	_, err := normalizeDesiredReplicas(map[uint64]string{
		1: "",
	}, 1)
	if err == nil {
		t.Fatal("expected missing target validation error")
	}
}

func TestNormalizeDesiredReplicasTrimsTargets(t *testing.T) {
	t.Parallel()

	desired, err := normalizeDesiredReplicas(map[uint64]string{
		1: "  127.0.0.1:63000  ",
	}, 1)
	if err != nil {
		t.Fatalf("normalize desired replicas: %v", err)
	}
	if got := desired[1]; got != "127.0.0.1:63000" {
		t.Fatalf("desired[1] = %q", got)
	}
}

func TestValidateDesiredReplicaTargetsRejectsRemovedReplica(t *testing.T) {
	t.Parallel()

	err := validateDesiredReplicaTargets(Membership{
		Removed: []uint64{2},
	}, map[uint64]string{
		2: "127.0.0.1:63001",
		3: "127.0.0.1:63002",
	})
	if err == nil {
		t.Fatal("expected removed replica validation error")
	}
}

func TestValidateDesiredReplicaTargetsRejectsTargetChange(t *testing.T) {
	t.Parallel()

	err := validateDesiredReplicaTargets(Membership{
		Nodes: map[uint64]string{
			2: "127.0.0.1:63001",
		},
	}, map[uint64]string{
		2: "127.0.0.1:63002",
	})
	if err == nil {
		t.Fatal("expected target change validation error")
	}
}

func TestEnsureReplicaAddableAllowsSameTarget(t *testing.T) {
	t.Parallel()

	ok, err := ensureReplicaAddable(map[uint64]string{
		2: "127.0.0.1:63001",
	}, nil, 2, "127.0.0.1:63001")
	if err != nil {
		t.Fatalf("ensure replica addable: %v", err)
	}
	if ok {
		t.Fatal("expected same target to be idempotent")
	}
}

func TestEnsureReplicaAddableAllowsAddNewReplica(t *testing.T) {
	t.Parallel()

	ok, err := ensureReplicaAddable(map[uint64]string{}, nil, 3, "127.0.0.1:63002")
	if err != nil {
		t.Fatalf("ensure replica addable: %v", err)
	}
	if !ok {
		t.Fatal("expected new replica to be addable")
	}
}

func TestMissingReplicasSortsReplicaIDs(t *testing.T) {
	t.Parallel()

	current := map[uint64]string{
		3: "127.0.0.1:63002",
	}
	desired := map[uint64]string{
		2: "127.0.0.1:63001",
		1: "127.0.0.1:63000",
	}

	replicas := missingReplicas(current, desired)
	got := make([]string, 0, len(replicas))
	for _, replica := range replicas {
		got = append(got, replica.Target)
	}
	if got[0] != "127.0.0.1:63000" || got[1] != "127.0.0.1:63001" {
		t.Fatalf("missing replicas targets = %+v, want ordered", got)
	}
}

func TestExtraReplicasSortsReplicaIDs(t *testing.T) {
	t.Parallel()

	current := map[uint64]string{
		3: "127.0.0.1:63002",
		1: "127.0.0.1:63000",
		2: "127.0.0.1:63001",
	}
	desired := map[uint64]string{
		1: "127.0.0.1:63000",
	}

	extras := extraReplicas(current, desired)
	expected := []uint64{2, 3}
	if len(extras) != len(expected) {
		t.Fatalf("extra replicas = %v, want %v", extras, expected)
	}
	for i, replicaID := range extras {
		if replicaID != expected[i] {
			t.Fatalf("extra replicas[%d] = %d, want %d", i, replicaID, expected[i])
		}
	}
}

func TestSortedRemovedReturnsOrderedValues(t *testing.T) {
	t.Parallel()

	removed := sortedRemoved(map[uint64]struct{}{
		3: {},
		1: {},
		2: {},
	})
	expected := []uint64{1, 2, 3}
	if len(removed) != len(expected) {
		t.Fatalf("removed = %v, want %v", removed, expected)
	}
	for i, replicaID := range removed {
		if replicaID != expected[i] {
			t.Fatalf("removed[%d] = %d, want %d", i, replicaID, expected[i])
		}
	}
}
