package handler

import "net/http"

func (s *Service) handleClusterStorageNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.engine == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage engine unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, s.engine.StorageNodeInfos())
}
