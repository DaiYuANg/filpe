package config

import "time"

func (cfg Config) RepairIntervalDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.RepairInterval)
	if err != nil {
		return 10 * time.Minute
	}
	return duration
}

func (cfg Config) RepairRetryBackoffDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.RepairRetryBackoff)
	if err != nil {
		return time.Second
	}
	return duration
}

func (cfg Config) DedupeIntervalDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.DedupeInterval)
	if err != nil {
		return 30 * time.Minute
	}
	return duration
}

func (cfg Config) IndexTimeoutDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.IndexTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return duration
}

func (cfg Config) IndexRetryBackoffDuration() time.Duration {
	duration, err := time.ParseDuration(cfg.IndexRetryBackoff)
	if err != nil {
		return time.Second
	}
	return duration
}
