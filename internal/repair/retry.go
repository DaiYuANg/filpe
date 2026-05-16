package repair

import (
	"context"
	"fmt"
	"time"

	"github.com/lyonbrown4d/maxio/object"
)

func repairObjectWithRetry(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
) (int, error) {
	attempts := max(runtime.cfg.RepairMaxRetries+1, 1)
	backoff := runtime.cfg.RepairRetryBackoffDuration()
	var repairErr error
	for attempt := range attempts {
		result, err := runtime.objects.RepairObject(ctx, meta.Bucket, meta.Key)
		if err == nil {
			return len(result.Repaired), nil
		}
		repairErr = err
		if attempt == attempts-1 {
			break
		}
		summary.RepairRetries++
		runtime.logger.DebugContext(ctx, "retry object repair",
			"bucket", meta.Bucket,
			"key", meta.Key,
			"attempt", attempt+1,
			"max_retries", runtime.cfg.RepairMaxRetries,
			"backoff", backoff.String(),
			"error", err,
		)
		if err := waitRepairRetry(ctx, backoff); err != nil {
			return 0, err
		}
	}
	return 0, repairErr
}

func waitRepairRetry(ctx context.Context, backoff time.Duration) error {
	if backoff <= 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait repair retry: %w", err)
		}
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait repair retry: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
