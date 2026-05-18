package repair

import (
	"fmt"
	"time"
)

func (runtime *Runtime) newRunID() string {
	return fmt.Sprintf("%s-%d", repairJobName, runtime.nextRunID.Add(1))
}

// History returns paginated completed repair records in newest-first order.
func (runtime *Runtime) History(offset, limit int) ([]RunRecord, int) {
	if runtime == nil {
		return nil, 0
	}
	if limit <= 0 {
		return nil, len(runtime.history)
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()

	total := len(runtime.history)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return nil, total
	}
	end := min(offset+limit, total)
	items := make([]RunRecord, end-offset)
	copy(items, runtime.history[offset:end])
	return items, total
}

// Issues returns paginated issues for a completed repair run.
func (runtime *Runtime) Issues(runID string, offset, limit int) ([]Issue, int, bool) {
	if runtime == nil || runID == "" {
		return nil, 0, false
	}
	if limit <= 0 {
		return nil, 0, true
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()

	allIssues, ok := runtime.issues[runID]
	if !ok {
		return nil, 0, false
	}
	total := len(allIssues)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []Issue{}, total, true
	}
	end := min(offset+limit, total)
	items := make([]Issue, end-offset)
	copy(items, allIssues[offset:end])
	return items, total, true
}

func (runtime *Runtime) recordRun(startedAt time.Time, runID string, summary Summary, err error) {
	if runtime == nil {
		return
	}
	if runID == "" {
		runID = runtime.newRunID()
	}
	if runtime.issues == nil {
		runtime.issues = make(map[string][]Issue)
	}
	record := RunRecord{
		RunID:      runID,
		StartedAt:  startedAt,
		FinishedAt: time.Now(),
		Duration:   runtime.status.LastDuration.Milliseconds(),
		Summary:    summary.withoutIssues(),
		IssueCount: len(summary.Issues),
	}
	if err != nil {
		record.Error = err.Error()
	}
	runtime.history = append([]RunRecord{record}, runtime.history...)
	if len(runtime.history) > maxRepairHistory {
		for i := maxRepairHistory; i < len(runtime.history); i++ {
			delete(runtime.issues, runtime.history[i].RunID)
		}
		runtime.history = runtime.history[:maxRepairHistory]
	}
	runtime.issues[runID] = append([]Issue(nil), summary.Issues...)
}
