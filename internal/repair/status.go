package repair

import "time"

// Status reports the most recent repair scan lifecycle state.
type Status struct {
	Running        bool      `json:"running"`
	LastStartedAt  time.Time `json:"last_started_at,omitzero"`
	LastFinishedAt time.Time `json:"last_finished_at,omitzero"`
	LastError      string    `json:"last_error,omitempty"`
	LastSummary    Summary   `json:"last_summary"`
}

func (runtime *Runtime) Status() Status {
	if runtime == nil {
		return Status{}
	}

	runtime.mu.RLock()
	defer runtime.mu.RUnlock()

	return runtime.status
}

func (runtime *Runtime) markStarted() {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	runtime.status.Running = true
	runtime.status.LastStartedAt = time.Now()
	runtime.status.LastError = ""
}

func (runtime *Runtime) markFinished(summary Summary, err error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	runtime.status.Running = false
	runtime.status.LastFinishedAt = time.Now()
	runtime.status.LastSummary = summary
	if err != nil {
		runtime.status.LastError = err.Error()
		return
	}
	runtime.status.LastError = ""
}
