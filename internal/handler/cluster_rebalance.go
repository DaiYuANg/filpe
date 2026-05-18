package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/lyonbrown4d/maxio/internal/model"
)

type rebalancePlanResponse struct {
	ReplicaID uint64 `json:"replica_id"`
	NodeID    string `json:"node_id"`
	Objects   int    `json:"objects"`
	Shards    int    `json:"shards"`
	UsedBytes int64  `json:"used_bytes"`
}

type rebalanceResponse struct {
	ReplicaID uint64 `json:"replica_id"`
	NodeID    string `json:"node_id"`
	Objects   int    `json:"objects"`
	Shards    int    `json:"shards"`
	UsedBytes int64  `json:"used_bytes"`
}

func (s *Service) handleClusterRebalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	replicaID, err := parseRequiredReplicaID(r)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.rebalanceClusterMember(r.Context(), replicaID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "cluster.rebalance",
		"replica_id", replicaID,
		"node_id", result.NodeID,
		"objects", result.Objects,
		"shards", result.Shards,
		"used_bytes", result.UsedBytes,
	)
	s.writeJSON(w, http.StatusAccepted, result)
}

func (s *Service) handleClusterRebalancePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	replicaID, err := parseRequiredReplicaID(r)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	plan, err := s.planClusterRebalance(r.Context(), replicaID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, plan)
}

func (s *Service) rebalanceClusterMember(ctx context.Context, replicaID uint64) (rebalanceResponse, error) {
	if s == nil || s.objects == nil {
		return rebalanceResponse{}, errors.New("object service unavailable")
	}
	nodeID := clusterStorageNodeID(replicaID)
	stats, err := s.countObjectPlacements(ctx, nodeID)
	if err != nil {
		return rebalanceResponse{}, err
	}
	result, err := s.objects.RebalanceNode(ctx, nodeID)
	if err != nil {
		return rebalanceResponse{}, fmt.Errorf("rebalance cluster member: %w", err)
	}
	return rebalanceResponse{
		ReplicaID: replicaID,
		NodeID:    result.NodeID,
		Objects:   result.Objects,
		Shards:    result.Shards,
		UsedBytes: stats.usedBytes,
	}, nil
}

func parseRequiredReplicaID(r *http.Request) (uint64, error) {
	value := r.URL.Query().Get("replica_id")
	if value == "" {
		return 0, errors.New("replica_id is required")
	}
	replicaID, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse replica_id: %w", err)
	}
	if replicaID == 0 {
		return 0, errors.New("replica_id must be greater than zero")
	}
	return replicaID, nil
}

func (s *Service) planClusterRebalance(ctx context.Context, replicaID uint64) (rebalancePlanResponse, error) {
	if s == nil || s.objects == nil {
		return rebalancePlanResponse{}, errors.New("object service unavailable")
	}
	nodeID := clusterStorageNodeID(replicaID)
	stats, err := s.countObjectPlacements(ctx, nodeID)
	if err != nil {
		return rebalancePlanResponse{}, err
	}
	return rebalancePlanResponse{
		ReplicaID: replicaID,
		NodeID:    nodeID,
		Objects:   stats.objects,
		Shards:    stats.shards,
		UsedBytes: stats.usedBytes,
	}, nil
}

type nodePlacementStats struct {
	objects   int
	shards    int
	usedBytes int64
}

func (s nodePlacementStats) hasPlacements() bool {
	return s.objects > 0 || s.shards > 0 || s.usedBytes > 0
}

func (s *Service) countObjectPlacements(ctx context.Context, nodeID string) (nodePlacementStats, error) {
	buckets, err := s.objects.ListBuckets(ctx)
	if err != nil {
		return nodePlacementStats{}, fmt.Errorf("list buckets for rebalance plan: %w", err)
	}
	var stats nodePlacementStats
	for _, bucket := range buckets {
		entries, err := s.objects.ListObjects(ctx, bucket.Name, "")
		if err != nil {
			return nodePlacementStats{}, fmt.Errorf("list objects for rebalance plan: %w", err)
		}
		stats.add(countObjectPlacements(entries, nodeID))
	}
	return stats, nil
}

func (s *nodePlacementStats) add(other nodePlacementStats) {
	s.objects += other.objects
	s.shards += other.shards
	s.usedBytes += other.usedBytes
}

func countObjectPlacements(objects []model.ObjectMeta, nodeID string) nodePlacementStats {
	var stats nodePlacementStats
	for index := range objects {
		object := &objects[index]
		matched := false
		for _, placement := range object.ShardPlacements {
			if placement.NodeID != nodeID {
				continue
			}
			matched = true
			stats.shards++
			stats.usedBytes += objectShardSize(object, placement.Index)
		}
		if matched {
			stats.objects++
		}
	}
	return stats
}

func objectShardSize(meta *model.ObjectMeta, shardIndex int) int64 {
	if meta == nil {
		return 0
	}
	if shardIndex < 0 {
		return 0
	}
	for index, size := range meta.ShardSizes {
		if index == shardIndex {
			return size
		}
	}
	return 0
}
