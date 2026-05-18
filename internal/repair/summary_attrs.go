package repair

func summaryAttrs(summary Summary) []any {
	return []any{
		"run_id", summary.RunID,
		"buckets", summary.Buckets,
		"objects", summary.Objects,
		"unhealthy", summary.Unhealthy,
		"missing", summary.Missing,
		"corrupted", summary.Corrupted,
		"scrubbed", summary.Scrubbed,
		"scrub_failed", summary.ScrubFailed,
		"checksum_failed", summary.ChecksumFailed,
		"repair_attempts", summary.RepairAttempts,
		"repair_retries", summary.RepairRetries,
		"retry_delay_ms", summary.RetryDelayMs,
		"throttled", summary.Throttled,
		"throttle_wait_ms", summary.ThrottleWaitMs,
		"repaired_objects", summary.RepairedObjects,
		"repaired_shards", summary.RepairedShards,
		"unrecoverable", summary.Unrecoverable,
		"failed", summary.Failed,
		"limited", summary.Limited,
	}
}
