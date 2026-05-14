package raft

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/arcgolabs/dix"
	dragonboat "github.com/lni/dragonboat/v4"
	dcfg "github.com/lni/dragonboat/v4/config"
	dbsm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/lyonbrown4d/maxio/internal/config"
)

type runtimeConfig struct {
	shardID      uint64
	replicaID    uint64
	raftAddress  string
	nodeHostDir  string
	rttMs        uint64
	electionRTT  uint64
	heartbeatRTT uint64
	deploymentID uint64
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

				rtCfg := &runtimeConfig{
					raftAddress:  cfg.RaftAddress,
					shardID:      cfg.RaftShardID,
					replicaID:    cfg.RaftNodeID,
					nodeHostDir:  nodeHostDir,
					rttMs:        200,
					electionRTT:  20,
					heartbeatRTT: 2,
				}
				if rtCfg.shardID == 0 {
					rtCfg.shardID = 1
				}
				if rtCfg.replicaID == 0 {
					rtCfg.replicaID = 1
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
				members := map[uint64]dragonboat.Target{
					rt.cfg.replicaID: dragonboat.Target(rt.cfg.raftAddress),
				}
				create := func(_, _ uint64) dbsm.IStateMachine {
					return newRaftStateMachine(rt.cfg.shardID, rt.cfg.replicaID)
				}
				if err := rt.node.StartReplica(members, false, create, startupConfig); err != nil {
					return fmt.Errorf("start raft replica: %w", err)
				}
				if rt.logger != nil {
					rt.logger.InfoContext(ctx, "dragonboat single-node started",
						"raft_address", rt.cfg.raftAddress,
						"shard_id", rt.cfg.shardID,
						"replica_id", rt.cfg.replicaID,
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
