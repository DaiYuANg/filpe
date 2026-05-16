package handler

import (
	"fmt"
	"net/http"
	"strconv"
)

func (s *Service) handleClusterMemberAction(w http.ResponseWriter, r *http.Request, replicaID, action string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseUint(replicaID, 10, 64)
	if err != nil {
		s.writeError(w, err)
		return
	}
	switch action {
	case "drain":
		s.handleDrainClusterMember(w, r, id)
	case "resume":
		s.handleResumeClusterMember(w, r, id)
	case "replace":
		s.handleReplaceClusterMember(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Service) handleDrainClusterMember(w http.ResponseWriter, r *http.Request, replicaID uint64) {
	if s.engine == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage engine unavailable"})
		return
	}
	nodeID := clusterStorageNodeID(replicaID)
	if err := s.engine.DrainStorageNode(nodeID); err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "cluster.member.drain", "replica_id", replicaID, "node_id", nodeID)
	s.writeJSON(w, http.StatusAccepted, map[string]any{
		"replica_id": replicaID,
		"node_id":    nodeID,
		"status":     "draining",
	})
}

func (s *Service) handleResumeClusterMember(w http.ResponseWriter, r *http.Request, replicaID uint64) {
	if s.engine == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage engine unavailable"})
		return
	}
	nodeID := clusterStorageNodeID(replicaID)
	if err := s.engine.ResumeStorageNode(nodeID); err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "cluster.member.resume", "replica_id", replicaID, "node_id", nodeID)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"replica_id": replicaID,
		"node_id":    nodeID,
		"status":     "active",
	})
}

func clusterStorageNodeID(replicaID uint64) string {
	return fmt.Sprintf("raft-%d", replicaID)
}
