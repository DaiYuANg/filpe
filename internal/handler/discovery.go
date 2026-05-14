package handler

import "net/http"

func (s *Service) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.discovery == nil {
		s.writeJSON(w, http.StatusOK, []any{})
		return
	}
	s.writeJSON(w, http.StatusOK, s.discovery.Nodes())
}
