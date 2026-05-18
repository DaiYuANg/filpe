package raft

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Membership struct {
	ConfigChangeID uint64            `json:"config_change_id"`
	LocalReplicaID uint64            `json:"local_replica_id"`
	Nodes          map[uint64]string `json:"nodes"`
	NonVotings     map[uint64]string `json:"non_votings"`
	Witnesses      map[uint64]string `json:"witnesses"`
	Removed        []uint64          `json:"removed"`
}

type Replica struct {
	ReplicaID uint64 `json:"replica_id"`
	Target    string `json:"target"`
}

type SyncMembershipResult struct {
	Before  Membership `json:"before"`
	After   Membership `json:"after"`
	Added   []Replica  `json:"added,omitempty"`
	Removed []uint64   `json:"removed,omitempty"`
}

func (rt *Runtime) GetMembership(ctx context.Context) (Membership, error) {
	if rt == nil || rt.node == nil {
		return Membership{}, errors.New("raft runtime is not ready")
	}
	ctx, cancel := rt.withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return Membership{}, fmt.Errorf("get raft membership: %w", err)
	}
	return Membership{
		ConfigChangeID: membership.ConfigChangeID,
		LocalReplicaID: rt.cfg.replicaID,
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

	ctx, cancel := rt.withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return fmt.Errorf("get raft membership before add replica: %w", err)
	}
	shouldAdd, err := ensureReplicaAddable(membership.Nodes, membership.Removed, replicaID, target)
	if err != nil {
		return err
	}
	if !shouldAdd {
		return nil
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

	ctx, cancel := rt.withRaftOperationTimeout(ctx)
	defer cancel()

	membership, err := rt.node.SyncGetShardMembership(ctx, rt.cfg.shardID)
	if err != nil {
		return fmt.Errorf("get raft membership before remove replica: %w", err)
	}
	if _, exists := membership.Nodes[replicaID]; !exists {
		return nil
	}
	if err := rt.node.SyncRequestDeleteReplica(ctx, rt.cfg.shardID, replicaID, membership.ConfigChangeID); err != nil {
		return fmt.Errorf("remove raft replica: %w", err)
	}
	return nil
}

func (rt *Runtime) SyncReplicas(ctx context.Context, desired map[uint64]string) (SyncMembershipResult, error) {
	if rt == nil || rt.node == nil {
		return SyncMembershipResult{}, errors.New("raft runtime is not ready")
	}
	normalized, err := normalizeDesiredReplicas(desired, rt.cfg.replicaID)
	if err != nil {
		return SyncMembershipResult{}, err
	}

	before, err := rt.GetMembership(ctx)
	if err != nil {
		return SyncMembershipResult{}, err
	}
	if validationErr := validateDesiredReplicaTargets(before, normalized); validationErr != nil {
		return SyncMembershipResult{Before: before}, validationErr
	}

	result := SyncMembershipResult{Before: before}
	if addErr := rt.addMissingReplicas(ctx, before.Nodes, normalized, &result); addErr != nil {
		return result, addErr
	}

	mid, err := rt.GetMembership(ctx)
	if err != nil {
		return result, err
	}
	if removeErr := rt.removeExtraReplicas(ctx, mid.Nodes, normalized, &result); removeErr != nil {
		return result, removeErr
	}

	after, err := rt.GetMembership(ctx)
	if err != nil {
		return result, err
	}
	result.After = after
	return result, nil
}

func (rt *Runtime) addMissingReplicas(
	ctx context.Context,
	current, desired map[uint64]string,
	result *SyncMembershipResult,
) error {
	for _, replica := range missingReplicas(current, desired) {
		if err := rt.AddReplica(ctx, replica.ReplicaID, replica.Target); err != nil {
			return fmt.Errorf("sync raft membership add replica %d: %w", replica.ReplicaID, err)
		}
		result.Added = append(result.Added, replica)
	}
	return nil
}

func (rt *Runtime) removeExtraReplicas(
	ctx context.Context,
	current, desired map[uint64]string,
	result *SyncMembershipResult,
) error {
	for _, replicaID := range extraReplicas(current, desired) {
		if err := rt.RemoveReplica(ctx, replicaID); err != nil {
			return fmt.Errorf("sync raft membership remove replica %d: %w", replicaID, err)
		}
		result.Removed = append(result.Removed, replicaID)
	}
	return nil
}

func normalizeDesiredReplicas(input map[uint64]string, localReplicaID uint64) (map[uint64]string, error) {
	if len(input) == 0 {
		return nil, errors.New("raft desired members are required")
	}
	desired := make(map[uint64]string, len(input))
	for replicaID, target := range input {
		target = strings.TrimSpace(target)
		if replicaID == 0 {
			return nil, errors.New("raft replica id must be greater than zero")
		}
		if target == "" {
			return nil, fmt.Errorf("raft replica target is required for replica %d", replicaID)
		}
		desired[replicaID] = target
	}
	if _, ok := desired[localReplicaID]; !ok {
		return nil, fmt.Errorf("raft desired members must include local replica %d", localReplicaID)
	}
	return desired, nil
}

func validateDesiredReplicaTargets(current Membership, desired map[uint64]string) error {
	for _, replicaID := range current.Removed {
		if _, ok := desired[replicaID]; ok {
			return fmt.Errorf("raft replica %d has been removed and cannot be added back", replicaID)
		}
	}
	for replicaID, target := range desired {
		if existing, ok := current.Nodes[replicaID]; ok && existing != target {
			return fmt.Errorf(
				"raft replica %d target cannot be changed from %q to %q; use replacement flow",
				replicaID,
				existing,
				target,
			)
		}
	}
	return nil
}

func ensureReplicaAddable(
	nodes map[uint64]string,
	removed map[uint64]struct{},
	replicaID uint64,
	target string,
) (bool, error) {
	if _, exists := removed[replicaID]; exists {
		return false, fmt.Errorf("raft replica %d has been removed and cannot be added back", replicaID)
	}
	if existing, exists := nodes[replicaID]; exists {
		if existing == target {
			return false, nil
		}
		return false, fmt.Errorf("raft replica %d target cannot be changed from %q to %q", replicaID, existing, target)
	}
	return true, nil
}

func missingReplicas(current, desired map[uint64]string) []Replica {
	replicas := make([]Replica, 0)
	for replicaID, target := range desired {
		if _, ok := current[replicaID]; !ok {
			replicas = append(replicas, Replica{ReplicaID: replicaID, Target: target})
		}
	}
	slices.SortFunc(replicas, func(left, right Replica) int {
		return cmp.Compare(left.ReplicaID, right.ReplicaID)
	})
	return replicas
}

func extraReplicas(current, desired map[uint64]string) []uint64 {
	replicaIDs := make([]uint64, 0)
	for replicaID := range current {
		if _, ok := desired[replicaID]; !ok {
			replicaIDs = append(replicaIDs, replicaID)
		}
	}
	slices.Sort(replicaIDs)
	return replicaIDs
}

func sortedRemoved(input map[uint64]struct{}) []uint64 {
	removed := make([]uint64, 0, len(input))
	for replicaID := range input {
		removed = append(removed, replicaID)
	}
	slices.Sort(removed)
	return removed
}
