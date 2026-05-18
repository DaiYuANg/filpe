package handler

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/raft"
)

func TestWriteErrorReturnsConflictForRaftNotLeader(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{}, slog.Default(), config.Config{})
	recorder := httptest.NewRecorder()
	service.writeError(recorder, raft.ErrNotLeader)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusConflict)
	}
	content := recorder.Body.String()
	if !strings.Contains(content, "not leader") {
		t.Fatalf("error response = %s", content)
	}
}

func TestWriteErrorReturnsConflictForRaftLeaderUnavailable(t *testing.T) {
	t.Parallel()

	service := NewService(Dependencies{}, slog.Default(), config.Config{})
	recorder := httptest.NewRecorder()
	service.writeError(recorder, raft.ErrLeaderUnavailable)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusConflict)
	}
	content := recorder.Body.String()
	if !strings.Contains(content, "leader unavailable") {
		t.Fatalf("error response = %s", content)
	}
}
