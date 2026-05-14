package handler

import (
	"encoding/json"
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
		membership, err := s.raft.GetMembership(r.Context())
		if err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusOK, membership)
	case http.MethodPost:
		var req addReplicaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, err)
			return
		}
		if err := s.raft.AddReplica(r.Context(), req.ReplicaID, req.Target); err != nil {
			s.writeError(w, err)
			return
		}
		s.writeJSON(w, http.StatusAccepted, map[string]any{
			"replica_id": req.ReplicaID,
			"target":     req.Target,
			"status":     "added",
		})
	case http.MethodPut:
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
		s.writeJSON(w, http.StatusOK, result)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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
	w.WriteHeader(http.StatusNoContent)
}
