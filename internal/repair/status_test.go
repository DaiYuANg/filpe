package repair

import "testing"

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
