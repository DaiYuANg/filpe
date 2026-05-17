package repair

import (
	"github.com/lyonbrown4d/maxio/object"
)

const maxRepairIssues = 50

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
