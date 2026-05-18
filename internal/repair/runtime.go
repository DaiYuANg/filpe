package repair

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/object"
)

func (runtime *Runtime) runOnce(ctx context.Context, runID, bucketFilter, prefixFilter string) (Summary, error) {
	if runtime == nil || runtime.objects == nil {
		return Summary{}, errors.New("repair runtime unavailable")
	}
	summary := Summary{RunID: runID}
	buckets, err := runtime.resolveBuckets(ctx, bucketFilter)
	if err != nil {
		return summary, fmt.Errorf("list buckets for repair: %w", err)
	}
	summary.Buckets = len(buckets)
	limiter := newRepairLimiter(runtime.cfg.RepairRateLimit)
	return runtime.scanBuckets(ctx, buckets, runID, strings.TrimSpace(prefixFilter), &summary, limiter)
}

func (runtime *Runtime) resolveBuckets(ctx context.Context, bucketFilter string) ([]object.Bucket, error) {
	if bucketFilter == "" {
		buckets, err := runtime.objects.ListBuckets(ctx)
		if err != nil {
			return nil, fmt.Errorf("list buckets for repair: %w", err)
		}
		return buckets, nil
	}
	return []object.Bucket{{Name: bucketFilter}}, nil
}

func (runtime *Runtime) scanBuckets(
	ctx context.Context,
	buckets []object.Bucket,
	runID string,
	prefixFilter string,
	summary *Summary,
	limiter *repairLimiter,
) (Summary, error) {
	sort.SliceStable(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})
	if summary == nil {
		summary = &Summary{RunID: runID}
	}
	totalBuckets := len(buckets)
	for idx, bucket := range buckets {
		if err := scanBucket(ctx, runtime, bucket, idx+1, totalBuckets, runID, prefixFilter, summary, limiter); err != nil {
			return *summary, err
		}
		if summary.Limited {
			break
		}
	}
	return *summary, nil
}

func scanBucket(
	ctx context.Context,
	runtime *Runtime,
	bucket object.Bucket,
	bucketIndex int,
	totalBuckets int,
	runID string,
	prefix string,
	summary *Summary,
	limiter *repairLimiter,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("scan repair bucket context: %w", err)
	}
	objects, err := runtime.objects.ListObjects(ctx, bucket.Name, prefix)
	if err != nil {
		summary.Failed++
		runtime.logger.WarnContext(ctx, "list objects for repair failed", "bucket", bucket.Name, "error", err)
		runtime.setProgress(RepairRunProgress{
			RunID:           runID,
			Bucket:          bucket.Name,
			BucketIndex:     bucketIndex,
			BucketTotal:     totalBuckets,
			ObjectIndex:     0,
			Object:          "",
			ObjectsScanned:  summary.Objects,
			ObjectsInBucket: 0,
		})
		return nil
	}
	runtime.setProgress(RepairRunProgress{
		RunID:           runID,
		Bucket:          bucket.Name,
		BucketIndex:     bucketIndex,
		BucketTotal:     totalBuckets,
		ObjectIndex:     0,
		Object:          "",
		ObjectsScanned:  summary.Objects,
		ObjectsInBucket: len(objects),
	})
	for idx := range objects {
		if shouldStopRepair(runtime.cfg, summary) {
			summary.Limited = true
			runtime.setProgress(RepairRunProgress{
				RunID:           runID,
				Bucket:          bucket.Name,
				BucketIndex:     bucketIndex,
				BucketTotal:     totalBuckets,
				ObjectIndex:     idx,
				ObjectsScanned:  summary.Objects,
				ObjectsInBucket: len(objects),
			})
			return nil
		}
		runtime.setProgress(RepairRunProgress{
			RunID:           runID,
			Bucket:          bucket.Name,
			BucketIndex:     bucketIndex,
			BucketTotal:     totalBuckets,
			ObjectIndex:     idx + 1,
			Object:          objects[idx].Key,
			ObjectsScanned:  summary.Objects,
			ObjectsInBucket: len(objects),
		})
		if err := repairObject(ctx, runtime, &objects[idx], summary, limiter); err != nil {
			return err
		}
		runtime.setProgress(RepairRunProgress{
			RunID:           runID,
			Bucket:          bucket.Name,
			BucketIndex:     bucketIndex,
			BucketTotal:     totalBuckets,
			ObjectIndex:     idx + 1,
			Object:          objects[idx].Key,
			ObjectsScanned:  summary.Objects,
			ObjectsInBucket: len(objects),
		})
	}
	return nil
}

func shouldStopRepair(cfg config.Config, summary *Summary) bool {
	return cfg.RepairMaxBatch > 0 && summary.RepairAttempts >= cfg.RepairMaxBatch
}

func repairObject(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
	limiter *repairLimiter,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("repair object context: %w", err)
	}
	summary.Objects++
	health, err := runtime.objects.CheckHealth(ctx, meta.Bucket, meta.Key)
	if err != nil {
		summary.Failed++
		addIssue(summary, Issue{
			Bucket: meta.Bucket,
			Key:    meta.Key,
			Kind:   IssueHealthCheckFailed,
			Reason: err.Error(),
		})
		runtime.logger.WarnContext(ctx, "check object health failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
		return nil
	}
	if health.Missing == 0 && health.Corrupted == 0 {
		return scrubHealthyObject(ctx, runtime, meta, summary)
	}
	summary.Missing += health.Missing
	summary.Corrupted += health.Corrupted
	summary.Unhealthy++
	if !health.Recoverable {
		summary.Unrecoverable++
		addIssue(summary, issueFromHealth(meta, health, IssueUnrecoverableShards, "object shards are not recoverable"))
		runtime.logger.WarnContext(ctx, "object shards are not recoverable",
			"bucket", meta.Bucket,
			"key", meta.Key,
			"missing", health.Missing,
			"corrupted", health.Corrupted,
			"available", health.Available,
			"total_chunks", health.TotalChunks,
		)
		return nil
	}
	throttled, waited, err := limiter.Wait(ctx)
	if err != nil {
		return err
	}
	if throttled {
		summary.Throttled++
		summary.ThrottleWaitMs += waited.Milliseconds()
	}
	summary.RepairAttempts++
	repairedShards, err := repairObjectWithRetry(ctx, runtime, meta, summary)
	if err != nil {
		summary.Failed++
		addIssue(summary, Issue{
			Bucket: meta.Bucket,
			Key:    meta.Key,
			Kind:   IssueRepairFailed,
			Reason: err.Error(),
		})
		runtime.logger.WarnContext(ctx, "repair object failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
		return nil
	}
	if repairedShards > 0 {
		summary.RepairedObjects++
		summary.RepairedShards += repairedShards
	}
	return nil
}
