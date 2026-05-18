package raft_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/raft"
)

func TestApplyStartupModeReuseRaftStateForJoin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	marker := filepath.Join(root, "seed")
	if err := os.WriteFile(marker, []byte("seed"), 0o600); err != nil {
		t.Fatalf("write raft state marker: %v", err)
	}

	cfg := config.Default()
	cfg.RaftDataDir = root
	cfg.RaftBootstrap = true
	cfg.RaftJoin = false

	join, err := raft.EffectiveRaftJoinMode(cfg)
	if err != nil {
		t.Fatalf("newRuntimeConfig: %v", err)
	}
	if !join {
		t.Fatalf("expected join mode after existing raft data is found")
	}
}

func TestApplyStartupModeKeepsBootstrapForFreshRaftDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	cfg := config.Default()
	cfg.RaftDataDir = root
	cfg.RaftBootstrap = true
	cfg.RaftJoin = false

	join, err := raft.EffectiveRaftJoinMode(cfg)
	if err != nil {
		t.Fatalf("newRuntimeConfig: %v", err)
	}
	if join {
		t.Fatalf("expected bootstrap mode for fresh raft data directory")
	}
}

func TestApplyStartupModeRebuildsBootstrapWhenInitialMembersProvided(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RaftDataDir = t.TempDir()
	cfg.RaftBootstrap = false
	cfg.RaftJoin = false
	cfg.RaftInitialMembers = "1=127.0.0.1:63001,2=127.0.0.1:63002"

	join, err := raft.EffectiveRaftJoinMode(cfg)
	if err != nil {
		t.Fatalf("newRuntimeConfig: %v", err)
	}
	if join {
		t.Fatalf("expected bootstrap mode for initial members on fresh raft data directory")
	}
}

func TestApplyStartupModeRejectsInitialMembersMissingLocalReplica(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RaftDataDir = t.TempDir()
	cfg.RaftBootstrap = false
	cfg.RaftJoin = false
	cfg.RaftInitialMembers = "2=127.0.0.1:63001,3=127.0.0.1:63002"

	_, err := raft.EffectiveRaftJoinMode(cfg)
	if err == nil {
		t.Fatalf("expected missing local replica error for initial members")
	}
}

func TestApplyStartupModeRequiresModeForFreshRaftDir(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RaftDataDir = t.TempDir()
	cfg.RaftBootstrap = false
	cfg.RaftJoin = false

	if _, err := raft.EffectiveRaftJoinMode(cfg); err == nil {
		t.Fatalf("expected startup mode resolution error for unspecified mode")
	}
}

func TestApplyStartupModePrefersJoinFlag(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RaftBootstrap = false
	cfg.RaftJoin = true

	join, err := raft.EffectiveRaftJoinMode(cfg)
	if err != nil {
		t.Fatalf("newRuntimeConfig: %v", err)
	}
	if !join {
		t.Fatalf("expected explicit join mode to be preserved")
	}
}

func TestApplyStartupModeKeepsJoinOverBootstrap(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.RaftDataDir = t.TempDir()
	cfg.RaftBootstrap = true
	cfg.RaftJoin = true

	join, err := raft.EffectiveRaftJoinMode(cfg)
	if err != nil {
		t.Fatalf("newRuntimeConfig: %v", err)
	}
	if !join {
		t.Fatalf("expected explicit join to win over bootstrap")
	}
}

func TestHasRaftState(t *testing.T) {
	t.Parallel()

	existingDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(existingDir, "state.marker"), []byte("marker"), 0o600); err != nil {
		t.Fatalf("write marker file: %v", err)
	}
	if ok, err := raft.HasRaftState(existingDir); err != nil {
		t.Fatalf("hasRaftState: %v", err)
	} else if !ok {
		t.Fatalf("expected existing data dir to contain state")
	}

	emptyDir := t.TempDir()
	if entries, err := os.ReadDir(emptyDir); err != nil {
		t.Fatalf("read empty dir: %v", err)
	} else if len(entries) > 0 {
		t.Fatalf("emptyDir should not contain files")
	}
	if ok, err := raft.HasRaftState(emptyDir); err != nil {
		t.Fatalf("hasRaftState: %v", err)
	} else if ok {
		t.Fatalf("expected empty data dir to be treated as fresh state")
	}
}
