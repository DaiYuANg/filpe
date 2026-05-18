package repair

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRepairLimiterWait(t *testing.T) {
	t.Parallel()

	limiter := &repairLimiter{interval: 10 * time.Millisecond}
	ctx := context.Background()

	throttled, wait, err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("first wait failed: %v", err)
	}
	if throttled {
		t.Fatalf("first wait must not be throttled")
	}
	if wait != 0 {
		t.Fatalf("first wait must not include delay, got %v", wait)
	}

	throttled, wait, err = limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("second wait failed: %v", err)
	}
	if !throttled {
		t.Fatalf("second wait expected throttling")
	}
	if wait <= 0 {
		t.Fatalf("second wait should return positive delay, got %v", wait)
	}
}

func TestRepairLimiterWaitContextCancelled(t *testing.T) {
	t.Parallel()

	limiter := &repairLimiter{interval: time.Minute}
	ctx := context.Background()
	if _, _, err := limiter.Wait(ctx); err != nil {
		t.Fatalf("prime wait failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}
