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
	collector.addDedupeStatus(s)
	collector.addIndexStatus(s)
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
	objects := 0
	shards := 0
	usedBytes := int64(0)
	for _, node := range nodes {
		if node.Drained {
			drained++
		}
		objects += node.ObjectCount
		shards += node.ShardCount
		usedBytes += node.UsedBytes
	}
	collector.gauge("maxio_storage_nodes", "Configured storage nodes.", len(nodes))
	collector.gauge("maxio_storage_nodes_drained", "Storage nodes excluded from new placements.", drained)
	collector.gauge("maxio_storage_node_objects", "Objects assigned to storage nodes.", objects)
	collector.gauge("maxio_storage_node_shards", "Shards assigned to storage nodes.", shards)
	collector.gaugeInt64("maxio_storage_node_used_bytes", "Bytes assigned to storage nodes.", usedBytes)
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
	collector.gauge("maxio_repair_last_scrubbed", "Shards scrubbed by the last repair job.", summary.Scrubbed)
	collector.gauge("maxio_repair_last_scrub_failed", "Scrub failures in the last repair job.", summary.ScrubFailed)
	collector.gauge("maxio_repair_last_checksum_failed", "Checksum failures in the last repair job.", summary.ChecksumFailed)
	collector.gauge("maxio_repair_last_repair_attempts", "Repair attempts in the last repair job.", summary.RepairAttempts)
	collector.gauge("maxio_repair_last_repair_retries", "Repair retries in the last repair job.", summary.RepairRetries)
	collector.gauge("maxio_repair_last_retry_delay_ms", "Retry delay milliseconds in the last repair job.", int(summary.RetryDelayMs))
	collector.gauge("maxio_repair_last_throttled", "Whether the last repair job was throttled.", summary.Throttled)
	collector.gauge("maxio_repair_last_throttle_wait_ms", "Throttle wait time in milliseconds in the last repair job.", int(summary.ThrottleWaitMs))
	collector.gauge("maxio_repair_last_repaired_objects", "Objects repaired by the last repair job.", summary.RepairedObjects)
	collector.gauge("maxio_repair_last_repaired_shards", "Shards repaired by the last repair job.", summary.RepairedShards)
	collector.gauge("maxio_repair_last_unrecoverable", "Unrecoverable items left by the last repair job.", summary.Unrecoverable)
	collector.gauge("maxio_repair_last_failed", "Failures recorded by the last repair job.", summary.Failed)
	collector.gauge("maxio_repair_last_limited", "Whether the last repair job was limited by configured thresholds.", boolInt(summary.Limited))
	collector.gauge("maxio_repair_last_duration_ms", "Milliseconds spent in last repair job.", int(status.LastDuration.Milliseconds()))
}

func (collector *metricsCollector) addDedupeStatus(s *Service) {
	if s == nil || s.dedupe == nil {
		collector.collectionErrors++
		return
	}
	status := s.dedupe.Status()
	result := status.LastResult
	collector.gauge("maxio_dedupe_running", "Whether the dedupe job is running.", boolInt(status.Running))
	collector.gauge("maxio_dedupe_last_objects", "Objects scanned by the last dedupe job.", result.Objects)
	collector.gauge("maxio_dedupe_last_blob_refs", "Blob refs scanned by the last dedupe job.", result.BlobRefs)
	collector.gauge("maxio_dedupe_last_hashes", "Unique object hashes seen by the last dedupe job.", result.Hashes)
	collector.gauge("maxio_dedupe_last_fixes", "Fixes applied by the last dedupe job.", result.Fixes)
	collector.gauge("maxio_dedupe_last_ref_count_drift", "Blob ref count drift found by the last dedupe job.", result.RefCountDrift)
	collector.gauge("maxio_dedupe_last_missing_blob_refs", "Missing blob refs found by the last dedupe job.", result.MissingBlobRefs)
	collector.gauge("maxio_dedupe_last_orphan_blob_refs", "Orphan blob refs found by the last dedupe job.", result.OrphanBlobRefs)
	collector.gauge("maxio_dedupe_last_layouts_mismatched", "Object layouts mismatched by the last dedupe job.", result.LayoutsMismatched)
	collector.gaugeInt64("maxio_dedupe_last_bytes_reclaimable", "Bytes reclaimable found by the last dedupe job.", result.BytesReclaimable)
	collector.gaugeInt64("maxio_dedupe_last_bytes_reclaimed", "Bytes reclaimed by the last dedupe job.", result.BytesReclaimed)
	collector.gauge("maxio_dedupe_last_limited", "Whether the last dedupe job was limited by configured thresholds.", boolInt(result.Limited))
}

func (collector *metricsCollector) addIndexStatus(s *Service) {
	if s == nil || s.objects == nil {
		collector.collectionErrors++
		return
	}
	status := s.objects.IndexStatus()
	collector.gauge("maxio_index_rebuilding", "Whether the content index rebuild is running.", boolInt(status.Rebuilding))
	collector.gauge("maxio_index_queue_size", "Configured content index queue size.", status.QueueSize)
	collector.gauge("maxio_index_queued_objects", "Objects waiting in the content index queue.", status.QueuedObjects)
	collector.gauge("maxio_index_dropped_objects", "Object index events dropped because the queue was full.", status.DroppedObjects)
	collector.gauge("maxio_index_retried_objects", "Object index tasks retried after failures.", status.RetriedObjects)
	collector.gauge("maxio_index_indexed_objects", "Objects successfully indexed by the content index worker.", status.IndexedObjects)
	collector.gauge("maxio_index_failed_objects", "Objects that failed content indexing.", status.FailedObjects)
	collector.gauge("maxio_index_last_rebuild_objects", "Objects indexed by the last content index rebuild.", status.LastRebuildObjects)
	collector.gauge("maxio_index_last_rebuild_failed", "Objects that failed during the last content index rebuild.", status.LastRebuildFailed)
}

func (collector *metricsCollector) gauge(name, help string, value int) {
	collector.gaugeInt64(name, help, int64(value))
}

func (collector *metricsCollector) gaugeInt64(name, help string, value int64) {
	collector.line("# HELP " + name + " " + help)
	collector.line("# TYPE " + name + " gauge")
	collector.line(name + " " + strconv.FormatInt(value, 10))
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
