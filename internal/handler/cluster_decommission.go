package handler

import (
	"context"
	"fmt"

	"github.com/lyonbrown4d/maxio/internal/model"
)

func (s *Service) ensureClusterMemberDecommissionable(ctx context.Context, replicaID uint64) error {
	if s == nil || s.objects == nil {
		return nil
	}
	nodeID := clusterStorageNodeID(replicaID)
	used, err := s.objectPlacementsUseNode(ctx, nodeID)
	if err != nil {
		return err
	}
	if used {
		return fmt.Errorf("replica %d still owns object shards; drain and rebalance before decommission", replicaID)
	}
	return nil
}

func (s *Service) objectPlacementsUseNode(ctx context.Context, nodeID string) (bool, error) {
	buckets, err := s.objects.ListBuckets(ctx)
	if err != nil {
		return false, fmt.Errorf("list buckets for decommission guard: %w", err)
	}
	for _, bucket := range buckets {
		objects, err := s.objects.ListObjects(ctx, bucket.Name, "")
		if err != nil {
			return false, fmt.Errorf("list objects for decommission guard: %w", err)
		}
		if objectPlacementsUseNode(objects, nodeID) {
			return true, nil
		}
	}
	return false, nil
}

func objectPlacementsUseNode(objects []model.ObjectMeta, nodeID string) bool {
	for index := range objects {
		objectMeta := &objects[index]
		for _, placement := range objectMeta.ShardPlacements {
			if placement.NodeID == nodeID {
				return true
			}
		}
	}
	return false
}
