package handler

import "net/http"

func (s *Service) handleRecoveryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.objects == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "object service unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, s.objects.RecoveryStatus())
}

func (s *Service) handleRecoveryRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.objects == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "object service unavailable"})
		return
	}
	result, err := s.objects.Recover(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "recovery.run",
		"pending_removed", result.PendingRemoved,
		"orphan_shard_sets_removed", result.OrphanShardCleanup.Removed,
	)
	s.writeJSON(w, http.StatusAccepted, result)
}
