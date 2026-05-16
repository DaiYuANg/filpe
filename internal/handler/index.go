package handler

import "net/http"

func (s *Service) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.objects == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "object service unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, s.objects.IndexStatus())
}

func (s *Service) handleIndexRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.objects == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "object service unavailable"})
		return
	}
	result, err := s.objects.RebuildIndex(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "index.rebuild", "objects", result.Objects, "failed", result.Failed)
	s.writeJSON(w, http.StatusAccepted, result)
}
