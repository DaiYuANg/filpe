package config

import (
	"errors"
	"fmt"
	"time"
)

func applyIndexZeroDefaults(cfg Config) Config {
	if cfg.IndexTimeout == "" {
		cfg.IndexTimeout = Default().IndexTimeout
	}
	if cfg.IndexRetryBackoff == "" {
		cfg.IndexRetryBackoff = Default().IndexRetryBackoff
	}
	if cfg.IndexMaxRetries < 0 {
		cfg.IndexMaxRetries = 0
	}
	if cfg.IndexQueueSize <= 0 {
		cfg.IndexQueueSize = Default().IndexQueueSize
	}
	return cfg
}

func validateIndexConfig(cfg Config) error {
	if _, err := time.ParseDuration(cfg.IndexTimeout); err != nil {
		return fmt.Errorf("invalid config: index_timeout: %w", err)
	}
	if _, err := time.ParseDuration(cfg.IndexRetryBackoff); err != nil {
		return fmt.Errorf("invalid config: index_retry_backoff: %w", err)
	}
	if cfg.IndexMaxRetries < 0 {
		return errors.New("invalid config: index_max_retries must be non-negative")
	}
	if cfg.IndexRateLimit < 0 {
		return errors.New("invalid config: index_rate_limit must be non-negative")
	}
	return nil
}
