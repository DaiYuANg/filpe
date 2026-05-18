package repair

import (
	"testing"
	"time"
)

func TestRepairRetryBackoffDuration(t *testing.T) {
	t.Parallel()

	base := 100 * time.Millisecond
	if got := repairRetryBackoffDuration(base, 0, 0, 2); got != 100*time.Millisecond {
		t.Fatalf("attempt 0 expected base backoff, got %v", got)
	}
	if got := repairRetryBackoffDuration(base, 1, 0, 2); got != 200*time.Millisecond {
		t.Fatalf("attempt 1 expected doubled backoff, got %v", got)
	}
	if got := repairRetryBackoffDuration(base, 2, 0, 2); got != 400*time.Millisecond {
		t.Fatalf("attempt 2 expected quadrupled backoff, got %v", got)
	}
}

func TestRepairRetryBackoffDurationClamp(t *testing.T) {
	t.Parallel()

	base := 200 * time.Millisecond
	maxBackoff := 350 * time.Millisecond
	if got := repairRetryBackoffDuration(base, 2, maxBackoff, 2); got != maxBackoff {
		t.Fatalf("backoff should be clamped to max, got %v", got)
	}
}
