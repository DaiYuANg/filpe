package handler

func (collector *metricsCollector) addHTTPMetrics(s *Service) {
	if s == nil || s.http == nil {
		collector.collectionErrors++
		return
	}
	snapshot := s.http.snapshot()
	collector.counterInt64("maxio_http_requests_total", "Total HTTP requests handled.", snapshot.Total)
	collector.counterInt64("maxio_http_errors_total", "Total HTTP requests completed with 4xx or 5xx status.", snapshot.Errors)
	collector.gaugeInt64("maxio_http_inflight_requests", "HTTP requests currently being handled.", snapshot.Inflight)
	collector.counterInt64("maxio_http_status_unknown_total", "Total HTTP requests with non-standard status codes.", snapshot.StatusClasses[0])
	collector.counterInt64("maxio_http_status_1xx_total", "Total HTTP requests completed with 1xx status.", snapshot.StatusClasses[1])
	collector.counterInt64("maxio_http_status_2xx_total", "Total HTTP requests completed with 2xx status.", snapshot.StatusClasses[2])
	collector.counterInt64("maxio_http_status_3xx_total", "Total HTTP requests completed with 3xx status.", snapshot.StatusClasses[3])
	collector.counterInt64("maxio_http_status_4xx_total", "Total HTTP requests completed with 4xx status.", snapshot.StatusClasses[4])
	collector.counterInt64("maxio_http_status_5xx_total", "Total HTTP requests completed with 5xx status.", snapshot.StatusClasses[5])
	collector.counterInt64("maxio_http_request_duration_ms_total", "Total HTTP request duration in milliseconds.", snapshot.DurationTotalMs)
	collector.gaugeInt64("maxio_http_request_duration_ms_max", "Maximum observed HTTP request duration in milliseconds.", snapshot.DurationMaxMs)
}

func (collector *metricsCollector) counterInt64(name, help string, value int64) {
	collector.line("# HELP " + name + " " + help)
	collector.line("# TYPE " + name + " counter")
	collector.line(name + " " + formatMetricInt(value))
}
