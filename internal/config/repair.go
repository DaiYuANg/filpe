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
