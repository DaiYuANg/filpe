package handler

import "net/http"

func (s *Service) auditHTTP(r *http.Request, action string, attrs ...any) {
	if s == nil || s.logger == nil || r == nil {
		return
	}
	fields := make([]any, 0, 8+len(attrs))
	fields = append(fields,
		"audit_action", action,
		"request_id", requestIDFromContext(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)
	fields = append(fields, attrs...)
	s.logger.InfoContext(r.Context(), "audit event", fields...)
}
