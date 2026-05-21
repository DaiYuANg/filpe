package handler

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type httpRequestMetrics struct {
	mu              sync.RWMutex
	total           int64
	errors          int64
	durationTotalMs int64
	durationMaxMs   int64
	statusClasses   [6]int64
}

type httpRequestMetricsSnapshot struct {
	Total           int64
	Errors          int64
	DurationTotalMs int64
	DurationMaxMs   int64
	StatusClasses   [6]int64
}

func newHTTPRequestMetrics() *httpRequestMetrics {
	return &httpRequestMetrics{}
}

func (metrics *httpRequestMetrics) record(status int, duration time.Duration) {
	if metrics == nil {
		return
	}
	durationMs := max(duration.Milliseconds(), int64(0))
	statusClass := httpStatusClass(status)

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	metrics.total++
	if status >= http.StatusBadRequest {
		metrics.errors++
	}
	metrics.durationTotalMs += durationMs
	if durationMs > metrics.durationMaxMs {
		metrics.durationMaxMs = durationMs
	}
	metrics.statusClasses[statusClass]++
}

func (metrics *httpRequestMetrics) snapshot() httpRequestMetricsSnapshot {
	if metrics == nil {
		return httpRequestMetricsSnapshot{}
	}
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	return httpRequestMetricsSnapshot{
		Total:           metrics.total,
		Errors:          metrics.errors,
		DurationTotalMs: metrics.durationTotalMs,
		DurationMaxMs:   metrics.durationMaxMs,
		StatusClasses:   metrics.statusClasses,
	}
}

func httpStatusClass(status int) int {
	if status < 100 || status >= 600 {
		return 0
	}
	return status / 100
}

func (s *Service) recordHTTPRequest(_ *http.Request, status int, duration time.Duration) {
	if s == nil || s.http == nil {
		return
	}
	s.http.record(status, duration)
}

type statusResponseWriter struct {
	http.ResponseWriter
	code int
}

func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{ResponseWriter: w}
}

func (w *statusResponseWriter) WriteHeader(code int) {
	if w.code != 0 {
		return
	}
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(data)
	if err != nil {
		return written, fmt.Errorf("write response: %w", err)
	}
	return written, nil
}

func (w *statusResponseWriter) status() int {
	if w.code == 0 {
		return http.StatusOK
	}
	return w.code
}
