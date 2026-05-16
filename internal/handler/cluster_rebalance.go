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
}

type rebalanceResponse struct {
	ReplicaID uint64 `json:"replica_id"`
	NodeID    string `json:"node_id"`
	Objects   int    `json:"objects"`
	Shards    int    `json:"shards"`
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
	result, err := s.objects.RebalanceNode(ctx, nodeID)
	if err != nil {
		return rebalanceResponse{}, fmt.Errorf("rebalance cluster member: %w", err)
	}
	return rebalanceResponse{
		ReplicaID: replicaID,
		NodeID:    result.NodeID,
		Objects:   result.Objects,
		Shards:    result.Shards,
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
	objects, shards, err := s.countObjectPlacements(ctx, nodeID)
	if err != nil {
		return rebalancePlanResponse{}, err
	}
	return rebalancePlanResponse{
		ReplicaID: replicaID,
		NodeID:    nodeID,
		Objects:   objects,
		Shards:    shards,
	}, nil
}

func (s *Service) countObjectPlacements(ctx context.Context, nodeID string) (int, int, error) {
	buckets, err := s.objects.ListBuckets(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list buckets for rebalance plan: %w", err)
	}
	objects := 0
	shards := 0
	for _, bucket := range buckets {
		entries, err := s.objects.ListObjects(ctx, bucket.Name, "")
		if err != nil {
			return 0, 0, fmt.Errorf("list objects for rebalance plan: %w", err)
		}
		objectCount, shardCount := countObjectPlacements(entries, nodeID)
		objects += objectCount
		shards += shardCount
	}
	return objects, shards, nil
}

func countObjectPlacements(objects []model.ObjectMeta, nodeID string) (int, int) {
	objectCount := 0
	shardCount := 0
	for index := range objects {
		matched := false
		for _, placement := range objects[index].ShardPlacements {
			if placement.NodeID != nodeID {
				continue
			}
			matched = true
			shardCount++
		}
		if matched {
			objectCount++
		}
	}
	return objectCount, shardCount
}
