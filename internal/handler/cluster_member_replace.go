package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lyonbrown4d/maxio/object"
)

type replacementReplicaRequest struct {
	ReplicaID uint64 `json:"replica_id"`
	Target    string `json:"target"`
}

type replacementReplicaResponse struct {
	OldReplicaID uint64 `json:"old_replica_id"`
	NewReplicaID uint64 `json:"new_replica_id"`
	Target       string `json:"target"`
	Objects      int    `json:"objects"`
	Shards       int    `json:"shards"`
	Status       string `json:"status"`
}

func (s *Service) handleReplaceClusterMember(w http.ResponseWriter, r *http.Request, oldReplicaID uint64) {
	req, err := decodeReplacementReplicaRequest(r)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := s.replaceClusterMember(r.Context(), oldReplicaID, req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "cluster.member.replace",
		"old_replica_id", oldReplicaID,
		"new_replica_id", req.ReplicaID,
		"target", req.Target,
		"objects", result.Objects,
		"shards", result.Shards,
	)
	s.writeJSON(w, http.StatusAccepted, result)
}

func decodeReplacementReplicaRequest(r *http.Request) (replacementReplicaRequest, error) {
	var req replacementReplicaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, fmt.Errorf("decode replacement replica request: %w", err)
	}
	req.Target = strings.TrimSpace(req.Target)
	if req.ReplicaID == 0 {
		return req, errors.New("replacement replica_id must be greater than zero")
	}
	if req.Target == "" {
		return req, errors.New("replacement target is required")
	}
	return req, nil
}

func (s *Service) replaceClusterMember(
	ctx context.Context,
	oldReplicaID uint64,
	req replacementReplicaRequest,
) (replacementReplicaResponse, error) {
	if oldReplicaID == req.ReplicaID {
		return replacementReplicaResponse{}, errors.New("replacement replica must be different from old replica")
	}
	if s == nil || s.raft == nil || s.engine == nil || s.objects == nil {
		return replacementReplicaResponse{}, errors.New("cluster replacement dependencies unavailable")
	}
	rebalance, err := s.runClusterMemberReplacement(ctx, oldReplicaID, req)
	if err != nil {
		return replacementReplicaResponse{}, err
	}
	return replacementReplicaResponse{
		OldReplicaID: oldReplicaID,
		NewReplicaID: req.ReplicaID,
		Target:       req.Target,
		Objects:      rebalance.Objects,
		Shards:       rebalance.Shards,
		Status:       "replaced",
	}, nil
}

func (s *Service) runClusterMemberReplacement(
	ctx context.Context,
	oldReplicaID uint64,
	req replacementReplicaRequest,
) (object.RebalanceResult, error) {
	if err := s.addReplacementReplica(ctx, req); err != nil {
		return object.RebalanceResult{}, err
	}
	oldNodeID := clusterStorageNodeID(oldReplicaID)
	rebalance, err := s.drainAndRebalanceOldReplica(ctx, oldNodeID)
	if err != nil {
		return object.RebalanceResult{}, err
	}
	if err := s.removeReplacedReplica(ctx, oldReplicaID); err != nil {
		return object.RebalanceResult{}, err
	}
	return rebalance, nil
}

func (s *Service) addReplacementReplica(ctx context.Context, req replacementReplicaRequest) error {
	if err := s.raft.AddReplica(ctx, req.ReplicaID, req.Target); err != nil {
		return fmt.Errorf("add replacement replica: %w", err)
	}
	if err := s.syncStorageNodes(ctx); err != nil {
		return fmt.Errorf("sync storage nodes after replacement add: %w", err)
	}
	return nil
}

func (s *Service) drainAndRebalanceOldReplica(ctx context.Context, oldNodeID string) (object.RebalanceResult, error) {
	if err := s.engine.DrainStorageNode(oldNodeID); err != nil {
		return object.RebalanceResult{}, fmt.Errorf("drain old replica storage node: %w", err)
	}
	rebalance, err := s.objects.RebalanceNode(ctx, oldNodeID)
	if err != nil {
		return object.RebalanceResult{}, fmt.Errorf("rebalance old replica: %w", err)
	}
	return rebalance, nil
}

func (s *Service) removeReplacedReplica(ctx context.Context, oldReplicaID uint64) error {
	if err := s.ensureClusterMemberDecommissionable(ctx, oldReplicaID); err != nil {
		return fmt.Errorf("verify old replica decommission: %w", err)
	}
	if err := s.raft.RemoveReplica(ctx, oldReplicaID); err != nil {
		return fmt.Errorf("remove old replica: %w", err)
	}
	if err := s.syncStorageNodes(ctx); err != nil {
		return fmt.Errorf("sync storage nodes after replacement remove: %w", err)
	}
	return nil
}
