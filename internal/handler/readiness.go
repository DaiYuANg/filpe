package handler

import (
	"context"
	"net/http"
)

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (s *Service) handleReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	response := s.readiness(r.Context())
	if response.Status != "ok" {
		s.writeJSON(w, http.StatusServiceUnavailable, response)
		return
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Service) readiness(ctx context.Context) readinessResponse {
	checks := map[string]string{}
	status := "ok"
	if err := s.checkReady(ctx, checks); err != nil {
		status = "not_ready"
	}
	return readinessResponse{Status: status, Checks: checks}
}

func (s *Service) checkReady(ctx context.Context, checks map[string]string) error {
	if s == nil {
		checks["service"] = "unavailable"
		return errReadinessUnavailable
	}
	err := s.checkObjectServiceReady(checks)
	err = joinReadiness(err, s.checkEngineReady(checks))
	err = joinReadiness(err, s.checkRaftReady(ctx, checks))
	return err
}

func (s *Service) checkObjectServiceReady(checks map[string]string) error {
	if s.objects == nil {
		checks["object_service"] = "unavailable"
		return errReadinessUnavailable
	}
	checks["object_service"] = "ok"
	return nil
}

func (s *Service) checkEngineReady(checks map[string]string) error {
	if s.engine == nil {
		checks["engine"] = "unavailable"
		return errReadinessUnavailable
	}
	if len(s.engine.StorageNodeInfos()) == 0 {
		checks["engine"] = "no_storage_nodes"
		return errReadinessUnavailable
	}
	checks["engine"] = "ok"
	return nil
}

func (s *Service) checkRaftReady(ctx context.Context, checks map[string]string) error {
	if s.raft == nil {
		checks["raft"] = "unavailable"
		return errReadinessUnavailable
	}
	if _, err := s.raft.GetMembership(ctx); err != nil {
		checks["raft"] = err.Error()
		return errReadinessUnavailable
	}
	checks["raft"] = "ok"
	return nil
}
