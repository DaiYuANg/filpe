package handler

import (
	"errors"
	"testing"

	raftx "github.com/lyonbrown4d/maxio/internal/raft"
)

func TestValidateClusterMemberReplacementAcceptsRemoteReplica(t *testing.T) {
	err := ValidateClusterMemberReplacement(2, raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "localhost:6301",
			2: "localhost:6302",
		},
	})

	if err != nil {
		t.Fatalf("validate replacement: %v", err)
	}
}

func TestValidateClusterMemberReplacementRejectsZeroReplica(t *testing.T) {
	err := ValidateClusterMemberReplacement(0, raftx.Membership{})

	if err == nil {
		t.Fatal("expected zero replica validation error")
	}
}

func TestValidateClusterMemberReplacementRejectsLocalReplica(t *testing.T) {
	err := ValidateClusterMemberReplacement(1, raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "localhost:6301",
			2: "localhost:6302",
		},
	})

	if !errors.Is(err, errCannotReplaceLocalReplica) {
		t.Fatalf("expected local replica error, got %v", err)
	}
}

func TestValidateClusterMemberReplacementRejectsMissingReplica(t *testing.T) {
	err := ValidateClusterMemberReplacement(3, raftx.Membership{
		LocalReplicaID: 1,
		Nodes: map[uint64]string{
			1: "localhost:6301",
			2: "localhost:6302",
		},
	})

	if !errors.Is(err, errClusterReplaceMemberNotFound) {
		t.Fatalf("expected missing member error, got %v", err)
	}
}
