package handler

import (
	"errors"
	"net/http"

	"github.com/lyonbrown4d/maxio/internal/repair"
)

func (s *Service) handleRepairStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.repair == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "repair runtime unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, s.repair.Status())
}

func (s *Service) handleRepairRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.repair == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "repair runtime unavailable"})
		return
	}
	summary, err := s.repair.RunOnce(r.Context())
	if errors.Is(err, repair.ErrRepairAlreadyRunning) {
		s.writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "repair.run", "buckets", summary.Buckets, "objects", summary.Objects, "repaired_shards", summary.RepairedShards, "failed", summary.Failed)
	s.writeJSON(w, http.StatusAccepted, summary)
}
