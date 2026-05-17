package s3bench

import (
	"math"
	"sync"
	"time"
)

type metrics struct {
	mu      sync.Mutex
	ops     map[string]*opStats
	errors  []string
	started time.Time
}

type opStats struct {
	Requests int
	Failed   int
	Bytes    int64
	Total    time.Duration
	Min      time.Duration
	Max      time.Duration
}

// Report is the JSON benchmark summary.
type Report struct {
	Endpoint           string              `json:"endpoint"`
	Bucket             string              `json:"bucket"`
	Objects            int                 `json:"objects"`
	Concurrency        int                 `json:"concurrency"`
	ObjectBytes        int                 `json:"object_bytes"`
	MultipartParts     int                 `json:"multipart_parts"`
	MultipartPartBytes int                 `json:"multipart_part_bytes"`
	DurationMS         int64               `json:"duration_ms"`
	Requests           int                 `json:"requests"`
	Failed             int                 `json:"failed"`
	RequestsPerSecond  float64             `json:"requests_per_second"`
	Ops                map[string]OpReport `json:"ops"`
	Errors             []string            `json:"errors,omitempty"`
}

// OpReport is a per-operation JSON benchmark summary.
type OpReport struct {
	Requests int     `json:"requests"`
	Failed   int     `json:"failed"`
	Bytes    int64   `json:"bytes"`
	AvgMS    float64 `json:"avg_ms"`
	MinMS    float64 `json:"min_ms"`
	MaxMS    float64 `json:"max_ms"`
}

func newMetrics() *metrics {
	return &metrics{
		ops:     make(map[string]*opStats),
		started: time.Now(),
	}
}

func (m *metrics) record(op string, elapsed time.Duration, byteCount int64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.ensureStatsLocked(op, time.Duration(math.MaxInt64))
	stats.Requests++
	stats.Total += elapsed
	stats.Bytes += max(byteCount, 0)
	if elapsed < stats.Min {
		stats.Min = elapsed
	}
	if elapsed > stats.Max {
		stats.Max = elapsed
	}
	if err != nil {
		stats.Failed++
		m.appendErrorLocked(op + ": " + err.Error())
	}
}

func (m *metrics) recordFailure(op, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.ensureStatsLocked(op, 0)
	stats.Failed++
	m.appendErrorLocked(op + ": " + message)
}

func (m *metrics) recordError(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendErrorLocked(message)
}

func (m *metrics) report(cfg Config) Report {
	m.mu.Lock()
	defer m.mu.Unlock()

	duration := time.Since(m.started)
	result := Report{
		Endpoint:           cfg.Endpoint,
		Bucket:             cfg.Bucket,
		Objects:            cfg.Objects,
		Concurrency:        cfg.Concurrency,
		ObjectBytes:        cfg.ObjectBytes,
		MultipartParts:     cfg.MultipartParts,
		MultipartPartBytes: cfg.MultipartPartBytes,
		DurationMS:         duration.Milliseconds(),
		Ops:                make(map[string]OpReport, len(m.ops)),
		Errors:             append([]string(nil), m.errors...),
	}
	for op, stats := range m.ops {
		result.Requests += stats.Requests
		result.Failed += stats.Failed
		result.Ops[op] = stats.report()
	}
	if duration > 0 {
		result.RequestsPerSecond = float64(result.Requests) / duration.Seconds()
	}
	return result
}

func (m *metrics) ensureStatsLocked(op string, initialMin time.Duration) *opStats {
	stats := m.ops[op]
	if stats == nil {
		stats = &opStats{Min: initialMin}
		m.ops[op] = stats
	}
	return stats
}

func (m *metrics) appendErrorLocked(message string) {
	if len(m.errors) < 20 {
		m.errors = append(m.errors, message)
	}
}

func (s opStats) report() OpReport {
	avg := time.Duration(0)
	if s.Requests > 0 {
		avg = s.Total / time.Duration(s.Requests)
	}
	minValue := s.Min
	if s.Requests == 0 || minValue == time.Duration(math.MaxInt64) {
		minValue = 0
	}
	return OpReport{
		Requests: s.Requests,
		Failed:   s.Failed,
		Bytes:    s.Bytes,
		AvgMS:    durationMS(avg),
		MinMS:    durationMS(minValue),
		MaxMS:    durationMS(s.Max),
	}
}
