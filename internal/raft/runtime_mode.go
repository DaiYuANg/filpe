package raft

import (
	"errors"
	"fmt"
	"os"

	"github.com/lyonbrown4d/maxio/internal/config"
)

func (cfg *runtimeConfig) applyStartupMode() error {
	if cfg == nil {
		return errors.New("raft runtime config is required")
	}
	if cfg.join {
		return nil
	}
	exists, err := hasRaftState(cfg.nodeHostDir)
	if err != nil {
		return fmt.Errorf("check raft startup mode: %w", err)
	}
	if exists {
		cfg.join = true
		cfg.bootstrap = false
		return nil
	}
	if len(cfg.initialMembers) > 0 {
		cfg.bootstrap = true
		return nil
	}
	if cfg.bootstrap {
		return nil
	}
	return errors.New("raft startup mode: set raft_bootstrap or raft_join true, or provide raft_initial_members for first boot")
}

func hasRaftState(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read raft data dir %q: %w", path, err)
	}
	return len(entries) > 0, nil
}

// EffectiveRaftJoinMode resolves whether the local replica should start in join mode
// based on persisted raft state and startup configuration.
func EffectiveRaftJoinMode(cfg config.Config) (bool, error) {
	rtCfg, err := newRuntimeConfig(cfg)
	if err != nil {
		return false, err
	}
	return rtCfg.join, nil
}

// HasRaftState reports whether the raft data directory contains persisted state.
func HasRaftState(path string) (bool, error) {
	return hasRaftState(path)
}
