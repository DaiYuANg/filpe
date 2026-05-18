package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/lyonbrown4d/maxio/internal/repair"
)

const (
	defaultRepairHistoryLimit = 20
	maxRepairHistoryLimit     = 100
	defaultRepairIssuesLimit  = 50
	maxRepairIssuesLimit      = 200
)

type repairHistoryResponse struct {
	Total  int                `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
	Runs   []repair.RunRecord `json:"runs"`
}

type repairIssuesResponse struct {
	RunID  string         `json:"run_id"`
	Total  int            `json:"total"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
	Issues []repair.Issue `json:"issues"`
}

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
	bucket, prefix := parseRepairRunScope(r)
	summary, err := s.repair.RunOnceWithScope(r.Context(), bucket, prefix)
	if errors.Is(err, repair.ErrRepairAlreadyRunning) {
		s.writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.auditHTTP(r, "repair.run",
		"run_id", summary.RunID,
		"buckets", summary.Buckets,
		"objects", summary.Objects,
		"repaired_shards", summary.RepairedShards,
		"failed", summary.Failed,
	)
	s.writeJSON(w, http.StatusAccepted, summary)
}

func (s *Service) handleRepairHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.repair == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "repair runtime unavailable"})
		return
	}
	limit, err := parseIntQueryParam(r, "limit", defaultRepairHistoryLimit, maxRepairHistoryLimit)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	offset, err := parseIntQueryParam(r, "offset", 0, 0)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	history, total := s.repair.History(offset, limit)
	s.writeJSON(w, http.StatusOK, repairHistoryResponse{
		Total:  total,
		Offset: offset,
		Limit:  limit,
		Runs:   history,
	})
}

func (s *Service) handleRepairIssues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.repair == nil {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "repair runtime unavailable"})
		return
	}
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if runID == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing run_id"})
		return
	}
	limit, err := parseIntQueryParam(r, "limit", defaultRepairIssuesLimit, maxRepairIssuesLimit)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	offset, err := parseIntQueryParam(r, "offset", 0, 0)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issues, total, found := s.repair.Issues(runID, offset, limit)
	if !found {
		s.writeJSON(w, http.StatusNotFound, map[string]string{"error": "repair run not found"})
		return
	}
	s.writeJSON(w, http.StatusOK, repairIssuesResponse{
		RunID:  runID,
		Total:  total,
		Offset: offset,
		Limit:  limit,
		Issues: issues,
	})
}

func parseRepairRunScope(r *http.Request) (string, string) {
	return strings.TrimSpace(r.URL.Query().Get("bucket")), strings.TrimSpace(r.URL.Query().Get("prefix"))
}

func parseIntQueryParam(r *http.Request, name string, defaultValue, maxValue int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse query %s: %w", name, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("query %s must be >= 0", name)
	}
	if maxValue > 0 && value > maxValue {
		return maxValue, nil
	}
	return value, nil
}
