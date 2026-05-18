package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type metricsCollector struct {
	builder          strings.Builder
	collectionErrors int
}

func (s *Service) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	output := s.collectMetrics(r.Context())
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(output)); err != nil {
		s.logger.WarnContext(r.Context(), "write metrics response failed", "error", err)
	}
}

func (s *Service) collectMetrics(ctx context.Context) string {
	collector := metricsCollector{}
	collector.addReadiness(ctx, s)
	collector.addStorageNodes(s)
	collector.addObjectCounts(ctx, s)
	collector.addRaftMembership(ctx, s)
	collector.addRepairStatus(s)
	collector.gauge("maxio_metrics_collection_errors", "Number of metric collection failures.", collector.collectionErrors)
	return collector.String()
}

func (collector *metricsCollector) addReadiness(ctx context.Context, s *Service) {
	value := 0
	if s.readiness(ctx).Status == "ok" {
		value = 1
	}
	collector.gauge("maxio_ready", "Whether MaxIO is ready to serve traffic.", value)
}

func (collector *metricsCollector) addStorageNodes(s *Service) {
	if s == nil || s.engine == nil {
		collector.collectionErrors++
		return
	}
	nodes := s.engine.StorageNodeInfos()
	drained := 0
	for _, node := range nodes {
		if node.Drained {
			drained++
		}
	}
	collector.gauge("maxio_storage_nodes", "Configured storage nodes.", len(nodes))
	collector.gauge("maxio_storage_nodes_drained", "Storage nodes excluded from new placements.", drained)
}

func (collector *metricsCollector) addObjectCounts(ctx context.Context, s *Service) {
	if s == nil || s.objects == nil {
		collector.collectionErrors++
		return
	}
	buckets, err := s.objects.ListBuckets(ctx)
	if err != nil {
		collector.collectionErrors++
		return
	}
	objects := 0
	for _, bucket := range buckets {
		items, err := s.objects.ListObjects(ctx, bucket.Name, "")
		if err != nil {
			collector.collectionErrors++
			continue
		}
		objects += len(items)
	}
	collector.gauge("maxio_buckets", "Buckets known to metadata.", len(buckets))
	collector.gauge("maxio_objects", "Committed objects known to metadata.", objects)
}

func (collector *metricsCollector) addRaftMembership(ctx context.Context, s *Service) {
	if s == nil || s.raft == nil {
		collector.collectionErrors++
		return
	}
	membership, err := s.raft.GetMembership(ctx)
	if err != nil {
		collector.collectionErrors++
		return
	}
	collector.gauge("maxio_raft_members", "Voting raft members.", len(membership.Nodes))
	collector.gauge("maxio_raft_removed_members", "Removed raft members.", len(membership.Removed))
}

func (collector *metricsCollector) addRepairStatus(s *Service) {
	if s == nil || s.repair == nil {
		collector.collectionErrors++
		return
	}
	status := s.repair.Status()
	summary := status.LastSummary
	collector.gauge("maxio_repair_running", "Whether the repair job is running.", boolInt(status.Running))
	collector.gauge("maxio_repair_last_objects", "Objects scanned by the last repair job.", summary.Objects)
	collector.gauge("maxio_repair_last_unhealthy", "Unhealthy objects found by the last repair job.", summary.Unhealthy)
	collector.gauge("maxio_repair_last_repaired_shards", "Shards repaired by the last repair job.", summary.RepairedShards)
	collector.gauge("maxio_repair_last_failed", "Failures recorded by the last repair job.", summary.Failed)
	collector.gauge("maxio_repair_last_duration_ms", "Milliseconds spent in last repair job.", int(status.LastDuration.Milliseconds()))
}

func (collector *metricsCollector) gauge(name, help string, value int) {
	collector.line("# HELP " + name + " " + help)
	collector.line("# TYPE " + name + " gauge")
	collector.line(name + " " + strconv.Itoa(value))
}

func (collector *metricsCollector) line(value string) {
	if _, err := collector.builder.WriteString(value); err != nil {
		collector.collectionErrors++
	}
	if err := collector.builder.WriteByte('\n'); err != nil {
		collector.collectionErrors++
	}
}

func (collector *metricsCollector) String() string {
	return collector.builder.String()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
