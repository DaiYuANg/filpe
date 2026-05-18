package repair

import (
	"context"
	"fmt"
	"time"
)

type repairLimiter struct {
	interval time.Duration
	next     time.Time
}

func newRepairLimiter(rateLimit int) *repairLimiter {
	if rateLimit <= 0 {
		return &repairLimiter{}
	}
	return &repairLimiter{interval: time.Second / time.Duration(rateLimit)}
}

func (limiter *repairLimiter) Wait(ctx context.Context) (bool, time.Duration, error) {
	if limiter == nil || limiter.interval <= 0 {
		return false, 0, nil
	}
	now := time.Now()
	if limiter.next.IsZero() {
		limiter.next = now.Add(limiter.interval)
		return false, 0, nil
	}
	wait := time.Until(limiter.next)
	if wait <= 0 {
		limiter.next = now.Add(limiter.interval)
		return false, 0, nil
	}
	if err := sleepRepairLimit(ctx, wait); err != nil {
		return false, 0, err
	}
	limiter.next = time.Now().Add(limiter.interval)
	return true, wait, nil
}

func sleepRepairLimit(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait repair rate limit: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
