package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/model"
)

type pendingCleanup struct {
	cfg    config.Config
	store  *Store
	logger *slog.Logger
}

func (c *pendingCleanup) run(ctx context.Context) error {
	if c.store == nil {
		return nil
	}
	result, err := c.store.Recover(ctx, RecoveryOptions{
		PendingTTL:          c.cfg.PendingObjectTTLDuration(),
		CleanupOrphanShards: true,
		Logger:              c.logger,
	})
	if err != nil {
		if c.logger != nil {
			c.logger.WarnContext(ctx, "store recovery failed", "error", err)
		}
		return nil
	}
	if c.logger != nil {
		c.logger.InfoContext(ctx, "store recovery completed",
			"pending_removed", result.PendingRemoved,
			"orphan_shard_sets_removed", result.OrphanShardCleanup.Removed,
			"orphan_shard_sets_scanned", result.OrphanShardCleanup.Scanned,
			"ttl", c.cfg.PendingObjectTTL,
		)
	}
	return nil
}

func (s *Store) CleanupPendingObjects(ctx context.Context, ttl time.Duration, logger *slog.Logger) (int, error) {
	if ttl <= 0 {
		return 0, nil
	}
	objects, err := s.meta.ListStagedObjectMetas(ctx, "", "")
	if err != nil {
		return 0, fmt.Errorf("list staged object metadata: %w", mapStoreError(err))
	}

	cutoff := time.Now().UTC().Add(-ttl)
	removed := 0
	for index := range objects {
		meta := objects[index]
		if !isExpiredPendingObject(meta, cutoff) {
			continue
		}
		exists, err := s.deleteExpiredPendingObject(ctx, meta, logger)
		if err != nil {
			return removed, err
		}
		if exists {
			removed++
		}
	}
	return removed, nil
}

func (s *Store) deleteExpiredPendingObject(ctx context.Context, meta model.ObjectMeta, logger *slog.Logger) (bool, error) {
	if _, exists, err := s.meta.DeleteStagedObjectMeta(ctx, meta.Bucket, meta.Key); err != nil {
		return false, fmt.Errorf("delete expired staged object metadata: %w", mapStoreError(err))
	} else if exists {
		logExpiredPendingObject(ctx, logger, meta)
		return true, nil
	}
	return false, nil
}

func logExpiredPendingObject(ctx context.Context, logger *slog.Logger, meta model.ObjectMeta) {
	if logger == nil {
		return
	}
	logger.WarnContext(ctx, "expired pending object metadata removed",
		"bucket", meta.Bucket,
		"key", meta.Key,
		"updated_at", meta.UpdatedAt,
	)
}

func isExpiredPendingObject(meta model.ObjectMeta, cutoff time.Time) bool {
	if meta.State != "" && meta.State != model.ObjectStatePending {
		return false
	}
	return meta.UpdatedAt.IsZero() || !meta.UpdatedAt.After(cutoff)
}
