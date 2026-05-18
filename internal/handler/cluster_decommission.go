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
		return fmt.Errorf("replica %d still owns object shards; drain and rebalance before decommission", replicaID)
	}
	return nil
}
