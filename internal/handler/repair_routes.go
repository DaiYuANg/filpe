package handler

import "net/http"

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
