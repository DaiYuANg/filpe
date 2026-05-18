package repair

import (
	"fmt"
	"testing"

	"github.com/lyonbrown4d/maxio/object"
)

func TestRuntimeSetProgress(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{}

	startedAt, _ := runtime.tryMarkStarted("run-1")
	runtime.setProgress(RepairRunProgress{
		RunID:       "run-1",
		Bucket:      "bucket-a",
		Object:      "object-1",
		ObjectIndex: 1,
	})

	status := runtime.Status()
	if status.Progress == nil {
		t.Fatal("progress should be set during running")
	}
	if status.Progress.RunID != "run-1" {
		t.Fatalf("expected runID run-1, got %q", status.Progress.RunID)
	}
	if status.Progress.Bucket != "bucket-a" {
		t.Fatalf("expected bucket bucket-a, got %q", status.Progress.Bucket)
	}

	runtime.markFinished(startedAt, "run-1", Summary{}, nil)
	if runtime.status.Progress != nil {
		t.Fatal("progress should be cleared after successful run")
	}
}

func TestRuntimeSetProgressRequiresRunningRun(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{}

	runtime.setProgress(RepairRunProgress{
		RunID:  "run-1",
		Bucket: "bucket-a",
	})

	status := runtime.Status()
	if status.Progress != nil {
		t.Fatalf("progress should not be set when runtime not running")
	}
}

func TestRuntimeMarkFinishedStoresRunIDInLastSummary(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{}

	startedAt, _ := runtime.tryMarkStarted("run-1")
	runtime.markFinished(startedAt, "run-1", Summary{RunID: "run-1", Buckets: 2}, nil)
	status := runtime.Status()

	if status.LastSummary.RunID != "run-1" {
		t.Fatalf("expected status last summary runID = run-1, got %q", status.LastSummary.RunID)
	}
	if status.LastSummary.Buckets != 2 {
		t.Fatalf("expected status last summary buckets = 2, got %d", status.LastSummary.Buckets)
	}
}

func TestRuntimeMarkFinishedStoresErrorKindAndTrigger(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{}
	runtime.setRunTrigger(repairRunTriggerManual)
	startedAt, _ := runtime.tryMarkStarted("run-1")
	markErr := fmt.Errorf("mark failed: %w", object.ErrNotFound)
	runtime.markFinished(startedAt, "run-1", Summary{}, markErr)

	status := runtime.Status()
	if status.LastErrorKind != repairErrorKindNotFound {
		t.Fatalf("expected last error kind %q, got %q", repairErrorKindNotFound, status.LastErrorKind)
	}
	if status.LastTrigger != repairRunTriggerManual {
		t.Fatalf("expected last trigger %q, got %q", repairRunTriggerManual, status.LastTrigger)
	}
	if status.LastError != markErr.Error() {
		t.Fatalf("expected last error %q, got %q", markErr, status.LastError)
	}
}

func TestRuntimeHistoryRecordsErrorKindAndTrigger(t *testing.T) {
	t.Parallel()

	runtime := &Runtime{}
	runtime.setRunTrigger(repairRunTriggerScheduled)
	startedAt, _ := runtime.tryMarkStarted("run-1")
	runtime.markFinished(startedAt, "run-1", Summary{}, nil)

	history, total := runtime.History(0, 10)
	if total != 1 {
		t.Fatalf("expected total history 1, got %d", total)
	}
	if len(history) != 1 {
		t.Fatalf("expected one history item, got %d", len(history))
	}
	if history[0].Trigger != repairRunTriggerScheduled {
		t.Fatalf("expected history trigger %q, got %q", repairRunTriggerScheduled, history[0].Trigger)
	}
	if history[0].ErrorKind != "" {
		t.Fatalf("expected no error kind for success, got %q", history[0].ErrorKind)
	}
}
