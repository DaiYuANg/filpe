package repair

import (
	"time"

	"github.com/lyonbrown4d/maxio/object"
)

const (
	maxRepairIssues  = 50
	maxRepairHistory = 20
)

// RunRecord reports one completed repair run.
type RunRecord struct {
	RunID      string    `json:"run_id"`
	StartedAt  time.Time `json:"started_at,omitzero"`
	FinishedAt time.Time `json:"finished_at,omitzero"`
	Duration   int64     `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
	Summary    Summary   `json:"summary"`
	IssueCount int       `json:"issue_count"`
}

const (
	IssueHealthCheckFailed   = "health_check_failed"
	IssueUnhealthyScrub      = "unhealthy_scrub"
	IssueChecksumFailed      = "checksum_failed"
	IssueRepairFailed        = "repair_failed"
	IssueScrubFailed         = "scrub_failed"
	IssueUnrecoverableShards = "unrecoverable_shards"
)

// Issue captures one object-level repair or scrub problem from the last run.
type Issue struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Reason      string `json:"reason,omitempty"`
	Missing     int    `json:"missing,omitempty"`
	Corrupted   int    `json:"corrupted,omitempty"`
	Available   int    `json:"available,omitempty"`
	TotalChunks int    `json:"total_chunks,omitempty"`
	Recoverable bool   `json:"recoverable"`
}

func addIssue(summary *Summary, issue Issue) {
	if summary == nil || len(summary.Issues) >= maxRepairIssues {
		return
	}
	summary.Issues = append(summary.Issues, issue)
}

func issueFromHealth(meta *object.ObjectMeta, health object.Health, kind, reason string) Issue {
	if meta == nil {
		return Issue{Kind: kind, Reason: reason}
	}
	return Issue{
		Bucket:      meta.Bucket,
		Key:         meta.Key,
		Kind:        kind,
		Reason:      reason,
		Missing:     health.Missing,
		Corrupted:   health.Corrupted,
		Available:   health.Available,
		TotalChunks: health.TotalChunks,
		Recoverable: health.Recoverable,
	}
}

func (summary Summary) withoutIssues() Summary {
	summaryCopy := summary
	summaryCopy.Issues = nil
	return summaryCopy
}
