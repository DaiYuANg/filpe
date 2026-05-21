package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	raftx "github.com/lyonbrown4d/maxio/internal/raft"
)

func TestReadinessReportsLeaderUnavailableDiagnostic(t *testing.T) {
	t.Parallel()

	raft := newLifecycleRaft(map[uint64]string{
		1: "127.0.0.1:63001",
		2: "127.0.0.1:63002",
	})
	raft.leaderErr = raftx.ErrLeaderUnavailable
	service := newLifecycleService(t, raft)

	response := service.readiness(context.Background())

	if response.Status != "not_ready" {
		t.Fatalf("status = %q, want not_ready", response.Status)
	}
	if response.Checks["raft_membership"] != "ok" {
		t.Fatalf("raft_membership = %q, want ok", response.Checks["raft_membership"])
	}
	if response.Checks["raft_leader"] != raftx.ErrLeaderUnavailable.Error() {
		t.Fatalf("raft_leader = %q, want %q", response.Checks["raft_leader"], raftx.ErrLeaderUnavailable.Error())
	}
}

func TestReadinessTreatsRemoteLeaderAsReadyDiagnostic(t *testing.T) {
	t.Parallel()

	raft := newLifecycleRaft(map[uint64]string{
		1: "127.0.0.1:63001",
		2: "127.0.0.1:63002",
	})
	raft.leaderErr = fmt.Errorf("%w: leader=2 local=1", raftx.ErrNotLeader)
	service := newLifecycleService(t, raft)

	response := service.readiness(context.Background())

	if response.Status != "ok" {
		t.Fatalf("status = %q, want ok", response.Status)
	}
	if response.Checks["raft_leader"] != "remote" {
		t.Fatalf("raft_leader = %q, want remote", response.Checks["raft_leader"])
	}
}

func TestClusterMetricsReportsLeaderUnavailableAndMembership(t *testing.T) {
	t.Parallel()

	raft := newLifecycleRaft(map[uint64]string{
		1: "127.0.0.1:63001",
		2: "127.0.0.1:63002",
	})
	raft.leaderErr = raftx.ErrLeaderUnavailable
	service := newLifecycleService(t, raft)
	collector := metricsCollector{}

	collector.addRaftStatus(context.Background(), service)

	output := collector.String()
	required := []string{
		"maxio_raft_local_replica_id 1",
		"maxio_raft_leader_available 0",
		"maxio_raft_local_is_leader 0",
		"maxio_raft_members 2",
	}
	for _, metric := range required {
		if !strings.Contains(output, metric) {
			t.Fatalf("expected metric %q in output, got: %s", metric, output)
		}
	}
}
