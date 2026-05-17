package repair

import (
	"context"
	"errors"

	"github.com/lyonbrown4d/maxio/object"
)

func scrubHealthyObject(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
) error {
	summary.Scrubbed++
	result, err := runtime.objects.ScrubObject(ctx, meta.Bucket, meta.Key)
	if err == nil && result.Healthy {
		return nil
	}
	if err == nil {
		recordUnhealthyScrub(ctx, runtime, meta, summary, result)
		return nil
	}
	recordFailedScrub(ctx, runtime, meta, summary, err)
	return nil
}

func recordUnhealthyScrub(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
	result object.ScrubResult,
) {
	summary.ScrubFailed++
	summary.Unhealthy++
	summary.Missing += result.Health.Missing
	summary.Corrupted += result.Health.Corrupted
	runtime.logger.WarnContext(ctx, "object scrub reported unhealthy shards",
		"bucket", meta.Bucket,
		"key", meta.Key,
		"missing", result.Health.Missing,
		"corrupted", result.Health.Corrupted,
	)
}

func recordFailedScrub(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
	err error,
) {
	summary.ScrubFailed++
	if errors.Is(err, object.ErrObjectCorrupted) {
		summary.Unhealthy++
		summary.Corrupted++
		summary.ChecksumFailed++
		summary.Unrecoverable++
		runtime.logger.WarnContext(ctx, "object checksum verification failed",
			"bucket", meta.Bucket,
			"key", meta.Key,
			"error", err,
		)
		return
	}
	summary.Failed++
	runtime.logger.WarnContext(ctx, "object scrub failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
}
