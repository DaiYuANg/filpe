package handler

import (
	"context"
	"fmt"
)

func (s *Service) ensureClusterMemberDecommissionable(ctx context.Context, replicaID uint64) error {
	if s == nil || s.objects == nil {
		return nil
	}
	nodeID := clusterStorageNodeID(replicaID)
	stats, err := s.countObjectPlacements(ctx, nodeID)
	if err != nil {
		return err
	}
	if stats.hasPlacements() {
		return &clusterDecommissionBlockedError{
			replicaID: replicaID,
			nodeID:    nodeID,
			stats:     stats,
		}
	}
	return nil
}

type clusterDecommissionBlockedError struct {
	replicaID uint64
	nodeID    string
	stats     nodePlacementStats
}

func (e *clusterDecommissionBlockedError) Error() string {
	return fmt.Sprintf(
		"replica %d still owns %d objects, %d shards, and %d bytes; drain and rebalance before decommission",
		e.replicaID,
		e.stats.objects,
		e.stats.shards,
		e.stats.usedBytes,
	)
}

func (e *clusterDecommissionBlockedError) Unwrap() error {
	return errClusterDecommissionBlocked
}
