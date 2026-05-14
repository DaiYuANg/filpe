package raft

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arcgolabs/dix"
	dragonboat "github.com/lni/dragonboat/v4"
	dcfg "github.com/lni/dragonboat/v4/config"
	dbsm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/lyonbrown4d/maxio/internal/config"
)

type runtimeConfig struct {
	shardID        uint64
	replicaID      uint64
	raftAddress    string
	nodeHostDir    string
	bootstrap      bool
	join           bool
	initialMembers map[uint64]string
	rttMs          uint64
	electionRTT    uint64
	heartbeatRTT   uint64
	deploymentID   uint64
}

type Runtime struct {
	cfg    *runtimeConfig
	node   *dragonboat.NodeHost
	logger *slog.Logger
}

func Module() dix.Module {
	return dix.NewModule(
		"raft",
		dix.WithModuleProviders(
			dix.ProviderErr2(func(cfg config.Config, logger *slog.Logger) (*Runtime, error) {
				nodeHostDir := cfg.RaftDataDir
				if nodeHostDir == "" {
					nodeHostDir = filepath.Join(cfg.DataDir, "raft")
				}
				configureDragonboatLogger(logger)
				initialMembers, err := parseInitialMembers(cfg.RaftInitialMembers)
				if err != nil {
					return nil, err
				}

				rtCfg := &runtimeConfig{
					raftAddress:    cfg.RaftAddress,
					shardID:        cfg.RaftShardID,
					replicaID:      cfg.RaftNodeID,
					nodeHostDir:    nodeHostDir,
					bootstrap:      cfg.RaftBootstrap,
					join:           cfg.RaftJoin,
					initialMembers: initialMembers,
					rttMs:          200,
					electionRTT:    20,
					heartbeatRTT:   2,
				}
				if rtCfg.shardID == 0 {
					rtCfg.shardID = 1
				}
				if rtCfg.replicaID == 0 {
					rtCfg.replicaID = 1
				}
				if err := validateInitialMembers(rtCfg); err != nil {
					return nil, err
				}

				nhc := dcfg.NodeHostConfig{
					NodeHostDir:    rtCfg.nodeHostDir,
					WALDir:         rtCfg.nodeHostDir,
					RTTMillisecond: rtCfg.rttMs,
					RaftAddress:    rtCfg.raftAddress,
					DeploymentID:   rtCfg.deploymentID,
				}
				nodeHost, err := dragonboat.NewNodeHost(nhc)
				if err != nil {
					return nil, fmt.Errorf("create dragonboat nodehost: %w", err)
				}
				return &Runtime{
					cfg:    rtCfg,
					node:   nodeHost,
					logger: logger,
				}, nil
			}),
		),
		dix.Hooks(
			dix.OnStart(func(ctx context.Context, rt *Runtime) error {
				if rt == nil || rt.node == nil {
					return nil
				}
				startupConfig := dcfg.Config{
					ReplicaID:    rt.cfg.replicaID,
					ShardID:      rt.cfg.shardID,
					HeartbeatRTT: rt.cfg.heartbeatRTT,
					ElectionRTT:  rt.cfg.electionRTT,
					CheckQuorum:  true,
				}
				members := rt.startupMembers()
				create := func(_, _ uint64) dbsm.IStateMachine {
					return newRaftStateMachine(rt.cfg.shardID, rt.cfg.replicaID)
				}
				if err := rt.node.StartReplica(members, rt.cfg.join, create, startupConfig); err != nil {
					return fmt.Errorf("start raft replica: %w", err)
				}
				if rt.logger != nil {
					rt.logger.InfoContext(ctx, "dragonboat replica started",
						"mode", rt.startupMode(),
						"raft_address", rt.cfg.raftAddress,
						"shard_id", rt.cfg.shardID,
						"replica_id", rt.cfg.replicaID,
						"initial_members", len(members),
					)
				}
				return nil
			}),
			dix.OnStop(func(_ context.Context, rt *Runtime) error {
				if rt == nil || rt.node == nil {
					return nil
				}
				if err := rt.node.StopShard(rt.cfg.shardID); err != nil && rt.logger != nil {
					rt.logger.Warn("stop raft shard failed", "error", err)
				}
				rt.node.Close()
				return nil
			}),
		),
	)
}

func (rt *Runtime) NodeHost() *dragonboat.NodeHost {
	if rt == nil {
		return nil
	}
	return rt.node
}

func (rt *Runtime) startupMembers() map[uint64]dragonboat.Target {
	if rt == nil || rt.cfg == nil || rt.cfg.join || !rt.cfg.bootstrap {
		return map[uint64]dragonboat.Target{}
	}
	if len(rt.cfg.initialMembers) == 0 {
		return map[uint64]dragonboat.Target{
			rt.cfg.replicaID: rt.cfg.raftAddress,
		}
	}
	members := make(map[uint64]dragonboat.Target, len(rt.cfg.initialMembers))
	for replicaID, target := range rt.cfg.initialMembers {
		members[replicaID] = target
	}
	return members
}

func (rt *Runtime) startupMode() string {
	if rt == nil || rt.cfg == nil {
		return "unknown"
	}
	if rt.cfg.join {
		return "join"
	}
	if rt.cfg.bootstrap {
		return "bootstrap"
	}
	return "restart"
}

func parseInitialMembers(value string) (map[uint64]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	members := make(map[uint64]string)
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		replicaID, target, err := parseInitialMember(item)
		if err != nil {
			return nil, err
		}
		if _, exists := members[replicaID]; exists {
			return nil, fmt.Errorf("duplicate raft initial member: %d", replicaID)
		}
		members[replicaID] = target
	}
	return members, nil
}

func parseInitialMember(value string) (uint64, string, error) {
	separator := strings.Index(value, "=")
	if separator < 0 {
		separator = strings.Index(value, "@")
	}
	if separator <= 0 || separator >= len(value)-1 {
		return 0, "", fmt.Errorf("invalid raft initial member %q, want id=address", value)
	}
	replicaID, err := strconv.ParseUint(strings.TrimSpace(value[:separator]), 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse raft initial member id: %w", err)
	}
	target := strings.TrimSpace(value[separator+1:])
	if replicaID == 0 {
		return 0, "", errors.New("raft initial member id must be greater than zero")
	}
	if target == "" {
		return 0, "", errors.New("raft initial member address is required")
	}
	return replicaID, target, nil
}

func validateInitialMembers(cfg *runtimeConfig) error {
	if cfg == nil || cfg.join || !cfg.bootstrap || len(cfg.initialMembers) == 0 {
		return nil
	}
	if _, ok := cfg.initialMembers[cfg.replicaID]; !ok {
		return fmt.Errorf("raft initial members must include local replica %d", cfg.replicaID)
	}
	return nil
}
