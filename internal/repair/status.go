package repair

import "time"

// Status reports the most recent repair scan lifecycle state.
type Status struct {
	Running        bool          `json:"running"`
	LastRunID      string        `json:"last_run_id,omitempty"`
	LastStartedAt  time.Time     `json:"last_started_at,omitzero"`
	LastFinishedAt time.Time     `json:"last_finished_at,omitzero"`
	LastDuration   time.Duration `json:"last_duration,omitzero"`
	LastError      string        `json:"last_error,omitempty"`
	LastSummary    Summary       `json:"last_summary"`
}

func (runtime *Runtime) Status() Status {
	if runtime == nil {
		return Status{}
	}

	runtime.mu.RLock()
	defer runtime.mu.RUnlock()

	return runtime.status
}

func (runtime *Runtime) tryMarkStarted(runID string) (time.Time, bool) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	if runtime.status.Running {
		return time.Time{}, false
	}

	runtime.status.Running = true
	runtime.status.LastRunID = runID
	now := time.Now()
	runtime.status.LastStartedAt = now
	runtime.status.LastError = ""
	return now, true
}

func (runtime *Runtime) markFinished(startedAt time.Time, runID string, summary Summary, err error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	runtime.status.Running = false
	runtime.status.LastFinishedAt = time.Now()
	runtime.status.LastRunID = runID
	runtime.status.LastSummary = summary
	if startedAt.IsZero() {
		runtime.status.LastDuration = 0
	} else {
		runtime.status.LastDuration = time.Since(startedAt)
	}
	runtime.recordRun(startedAt, runID, summary, err)
	if err != nil {
		runtime.status.LastError = err.Error()
		return
	}
	runtime.status.LastError = ""
}
