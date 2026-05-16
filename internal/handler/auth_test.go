package handler_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/handler"
)

func TestAdminAuthDisabledByDefault(t *testing.T) {
	recorder := serveHandlerGet(t, config.Config{}, "/metrics", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAdminAuthProtectsControlRoutes(t *testing.T) {
	cfg := config.Config{AdminToken: "secret"}
	recorder := serveHandlerGet(t, cfg, "/metrics", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAdminAuthAcceptsBearerCredential(t *testing.T) {
	headers := map[string]string{"Authorization": "Bearer secret"}
	recorder := serveHandlerGet(t, config.Config{AdminToken: "secret"}, "/metrics", headers)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAdminAuthAcceptsControlHeader(t *testing.T) {
	headers := map[string]string{"X-Maxio-Control": "secret"}
	recorder := serveHandlerGet(t, config.Config{AdminToken: "secret"}, "/metrics", headers)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestAdminAuthDoesNotProtectHealth(t *testing.T) {
	recorder := serveHandlerGet(t, config.Config{AdminToken: "secret"}, "/healthz", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func serveHandlerGet(
	t *testing.T,
	cfg config.Config,
	target string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	service := handler.NewService(handler.Dependencies{}, slog.New(slog.DiscardHandler), cfg)
	router := http.NewServeMux()
	service.RegisterHTTP(router)

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
