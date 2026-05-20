package handler

import (
	"strings"
	"testing"

	"log/slog"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/dedupe"
	"github.com/lyonbrown4d/maxio/internal/repair"
)

func TestAddRepairStatusAddsSummaryMetrics(t *testing.T) {
	t.Parallel()

	collector := metricsCollector{}
	service := newService(Dependencies{}, slog.Default(), config.Config{}, nil)
	service.repair = &repair.Runtime{}

	collector.addRepairStatus(service)
	output := collector.String()

	required := []string{
		"maxio_repair_last_scrubbed 0",
		"maxio_repair_last_scrub_failed 0",
		"maxio_repair_last_checksum_failed 0",
		"maxio_repair_last_repair_attempts 0",
		"maxio_repair_last_repair_retries 0",
		"maxio_repair_last_retry_delay_ms 0",
		"maxio_repair_last_throttled 0",
		"maxio_repair_last_throttle_wait_ms 0",
		"maxio_repair_last_repaired_objects 0",
		"maxio_repair_last_unrecoverable 0",
		"maxio_repair_last_limited 0",
	}

	for _, name := range required {
		if !strings.Contains(output, name) {
			t.Fatalf("expected metric %q in output, got: %s", name, output)
		}
	}
}

func TestAddDedupeStatusAddsSummaryMetrics(t *testing.T) {
	t.Parallel()

	collector := metricsCollector{}
	service := newService(Dependencies{}, slog.Default(), config.Config{}, &dedupe.Runtime{})

	collector.addDedupeStatus(service)
	output := collector.String()

	required := []string{
		"maxio_dedupe_running 0",
		"maxio_dedupe_last_objects 0",
		"maxio_dedupe_last_blob_refs 0",
		"maxio_dedupe_last_hashes 0",
		"maxio_dedupe_last_fixes 0",
		"maxio_dedupe_last_ref_count_drift 0",
		"maxio_dedupe_last_missing_blob_refs 0",
		"maxio_dedupe_last_orphan_blob_refs 0",
		"maxio_dedupe_last_layouts_mismatched 0",
		"maxio_dedupe_last_bytes_reclaimable 0",
		"maxio_dedupe_last_bytes_reclaimed 0",
		"maxio_dedupe_last_limited 0",
	}

	for _, name := range required {
		if !strings.Contains(output, name) {
			t.Fatalf("expected metric %q in output, got: %s", name, output)
		}
	}
}
