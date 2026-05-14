package raft

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Membership struct {
	ConfigChangeID uint64            `json:"config_change_id"`
	Nodes          map[uint64]string `json:"nodes"`
	NonVotings     map[uint64]string `json:"non_votings"`
	Witnesses      map[uint64]string `json:"witnesses"`
	Removed        []uint64          `json:"removed"`
}

func (rt *Runtime) GetMembership(ctx context.Context) (Membership, error) {
	if rt == nil || rt.node == nil {
		return Membership{}, errors.New("raft runtime is not ready")
	}
	ctx, cancel := withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return Membership{}, fmt.Errorf("get raft membership: %w", err)
	}
	return Membership{
		ConfigChangeID: membership.ConfigChangeID,
		Nodes:          maps.Clone(membership.Nodes),
		NonVotings:     maps.Clone(membership.NonVotings),
		Witnesses:      maps.Clone(membership.Witnesses),
		Removed:        sortedRemoved(membership.Removed),
	}, nil
}

func (rt *Runtime) AddReplica(ctx context.Context, replicaID uint64, target string) error {
	if rt == nil || rt.node == nil {
		return errors.New("raft runtime is not ready")
	}
	if replicaID == 0 {
		return errors.New("raft replica id must be greater than zero")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("raft replica target is required")
	}

	ctx, cancel := withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return fmt.Errorf("get raft membership before add replica: %w", err)
	}
	if err := rt.node.SyncRequestAddReplica(ctx, rt.cfg.shardID, replicaID, target, membership.ConfigChangeID); err != nil {
		return fmt.Errorf("add raft replica: %w", err)
	}
	return nil
}

func (rt *Runtime) RemoveReplica(ctx context.Context, replicaID uint64) error {
	if rt == nil || rt.node == nil {
		return errors.New("raft runtime is not ready")
	}
	if replicaID == 0 {
		return errors.New("raft replica id must be greater than zero")
	}

	ctx, cancel := withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return fmt.Errorf("get raft membership before remove replica: %w", err)
	}
	if err := rt.node.SyncRequestDeleteReplica(ctx, rt.cfg.shardID, replicaID, membership.ConfigChangeID); err != nil {
		return fmt.Errorf("remove raft replica: %w", err)
	}
	return nil
}

func sortedRemoved(input map[uint64]struct{}) []uint64 {
	removed := make([]uint64, 0, len(input))
	for replicaID := range input {
		removed = append(removed, replicaID)
	}
	slices.Sort(removed)
	return removed
}
