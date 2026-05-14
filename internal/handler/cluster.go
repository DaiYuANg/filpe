package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

type addReplicaRequest struct {
	ReplicaID uint64 `json:"replica_id"`
	Target    string `json:"target"`
}

type syncReplicasRequest struct {
	Nodes map[uint64]string `json:"nodes"`
}

func (s *Service) handleClusterMembers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListClusterMembers(w, r)
	case http.MethodPost:
		s.handleAddClusterMember(w, r)
	case http.MethodPut:
		s.handleSyncClusterMembers(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleListClusterMembers(w http.ResponseWriter, r *http.Request) {
	membership, err := s.raft.GetMembership(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, membership)
}

func (s *Service) handleAddClusterMember(w http.ResponseWriter, r *http.Request) {
	var req addReplicaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.raft.AddReplica(r.Context(), req.ReplicaID, req.Target); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.syncStorageNodes(r.Context()); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]any{
		"replica_id": req.ReplicaID,
		"target":     req.Target,
		"status":     "added",
	})
}

func (s *Service) handleSyncClusterMembers(w http.ResponseWriter, r *http.Request) {
	var req syncReplicasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, err)
		return
	}
	result, err := s.raft.SyncReplicas(r.Context(), req.Nodes)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.syncStorageNodes(r.Context()); err != nil {
		s.writeError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleClusterMember(w http.ResponseWriter, r *http.Request, replicaID string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseUint(replicaID, 10, 64)
	if err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.raft.RemoveReplica(r.Context(), id); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.syncStorageNodes(r.Context()); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) syncStorageNodes(ctx context.Context) error {
	if s == nil || s.engine == nil || s.raft == nil {
		return nil
	}

	membership, err := s.raft.GetMembership(ctx)
	if err != nil {
		return fmt.Errorf("get raft membership: %w", err)
	}
	localReplicaID := s.raft.LocalReplicaID()
	if localReplicaID == 0 {
		return errors.New("local raft replica id is missing")
	}
	if err := s.engine.SyncStorageNodesFromRaft(localReplicaID, membership.Nodes); err != nil {
		return fmt.Errorf("sync engine storage nodes: %w", err)
	}
	return nil
}
