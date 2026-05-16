// Package repair schedules and runs background object shard repair.
package repair

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/arcgolabs/dix"
	gocron "github.com/go-co-op/gocron/v2"
	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/scheduler"
	"github.com/lyonbrown4d/maxio/object"
)

const repairJobName = "maxio.object.repair"

// Summary reports one repair scan result.
type Summary struct {
	Buckets         int  `json:"buckets"`
	Objects         int  `json:"objects"`
	Unhealthy       int  `json:"unhealthy"`
	Missing         int  `json:"missing"`
	Corrupted       int  `json:"corrupted"`
	RepairAttempts  int  `json:"repair_attempts"`
	RepairRetries   int  `json:"repair_retries"`
	Throttled       int  `json:"throttled"`
	RepairedObjects int  `json:"repaired_objects"`
	RepairedShards  int  `json:"repaired_shards"`
	Unrecoverable   int  `json:"unrecoverable"`
	Failed          int  `json:"failed"`
	Limited         bool `json:"limited"`
}

// Runtime owns the scheduled repair job.
type Runtime struct {
	cfg       config.Config
	objects   *object.Service
	scheduler *scheduler.Runtime
	logger    *slog.Logger
	mu        sync.RWMutex
	status    Status
}

func Module() dix.Module {
	return dix.NewModule(
		"repair",
		dix.WithModuleProviders(
			dix.Provider4(NewRuntime),
		),
		dix.Hooks(
			dix.OnStart(startRuntime),
		),
	)
}

func NewRuntime(
	cfg config.Config,
	objects *object.Service,
	schedulerRuntime *scheduler.Runtime,
	logger *slog.Logger,
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{
		cfg:       cfg,
		objects:   objects,
		scheduler: schedulerRuntime,
		logger:    logger,
	}
}

func startRuntime(ctx context.Context, runtime *Runtime) error {
	if runtime == nil {
		return nil
	}
	return runtime.Start(ctx)
}

func (runtime *Runtime) Start(ctx context.Context) error {
	interval := runtime.cfg.RepairIntervalDuration()
	if _, err := runtime.scheduler.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(runtime.runScheduled),
		gocron.WithName(repairJobName),
		gocron.WithTags("repair", "storage"),
	); err != nil {
		return fmt.Errorf("schedule repair job: %w", err)
	}
	runtime.logger.InfoContext(ctx, "object repair job scheduled",
		"job", repairJobName,
		"interval", interval.String(),
		"max_batch", runtime.cfg.RepairMaxBatch,
	)
	if runtime.cfg.RepairOnStart {
		runtime.startInitialRepair(ctx)
	}
	return nil
}

func (runtime *Runtime) startInitialRepair(ctx context.Context) {
	if ctx == nil {
		runtime.logger.Warn("skip initial repair: nil context")
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		if err := runtime.scheduler.RequireLeader(runCtx); err != nil {
			runtime.logger.DebugContext(runCtx, "skip initial repair on non-leader", "error", err)
			return
		}
		runtime.runScheduled(runCtx)
	}()
}

func (runtime *Runtime) runScheduled(ctx context.Context) {
	summary, err := runtime.RunOnce(ctx)
	if err != nil {
		runtime.logger.ErrorContext(ctx, "object repair job failed", "error", err)
		return
	}
	attrs := summaryAttrs(summary)
	if summary.Unhealthy > 0 || summary.Failed > 0 {
		runtime.logger.InfoContext(ctx, "object repair job completed", attrs...)
		return
	}
	runtime.logger.DebugContext(ctx, "object repair job completed", attrs...)
}

func summaryAttrs(summary Summary) []any {
	return []any{
		"buckets", summary.Buckets,
		"objects", summary.Objects,
		"unhealthy", summary.Unhealthy,
		"missing", summary.Missing,
		"corrupted", summary.Corrupted,
		"repair_attempts", summary.RepairAttempts,
		"repair_retries", summary.RepairRetries,
		"throttled", summary.Throttled,
		"repaired_objects", summary.RepairedObjects,
		"repaired_shards", summary.RepairedShards,
		"unrecoverable", summary.Unrecoverable,
		"failed", summary.Failed,
		"limited", summary.Limited,
	}
}

func (runtime *Runtime) RunOnce(ctx context.Context) (Summary, error) {
	if runtime == nil {
		return Summary{}, errors.New("repair runtime unavailable")
	}
	runtime.markStarted()

	summary, err := runtime.runOnce(ctx)
	runtime.markFinished(summary, err)

	return summary, err
}

func (runtime *Runtime) runOnce(ctx context.Context) (Summary, error) {
	if runtime == nil || runtime.objects == nil {
		return Summary{}, errors.New("repair runtime unavailable")
	}
	buckets, err := runtime.objects.ListBuckets(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("list buckets for repair: %w", err)
	}
	summary := Summary{Buckets: len(buckets)}
	limiter := newRepairLimiter(runtime.cfg.RepairRateLimit)
	for _, bucket := range buckets {
		if err := scanBucket(ctx, runtime, bucket, &summary, limiter); err != nil {
			return summary, err
		}
		if summary.Limited {
			return summary, nil
		}
	}
	return summary, nil
}

func scanBucket(
	ctx context.Context,
	runtime *Runtime,
	bucket object.Bucket,
	summary *Summary,
	limiter *repairLimiter,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("scan repair bucket context: %w", err)
	}
	objects, err := runtime.objects.ListObjects(ctx, bucket.Name, "")
	if err != nil {
		summary.Failed++
		runtime.logger.WarnContext(ctx, "list objects for repair failed", "bucket", bucket.Name, "error", err)
		return nil
	}
	for idx := range objects {
		if shouldStopRepair(runtime.cfg, summary) {
			summary.Limited = true
			return nil
		}
		if err := repairObject(ctx, runtime, &objects[idx], summary, limiter); err != nil {
			return err
		}
	}
	return nil
}

func shouldStopRepair(cfg config.Config, summary *Summary) bool {
	return cfg.RepairMaxBatch > 0 && summary.RepairAttempts >= cfg.RepairMaxBatch
}

func repairObject(
	ctx context.Context,
	runtime *Runtime,
	meta *object.ObjectMeta,
	summary *Summary,
	limiter *repairLimiter,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("repair object context: %w", err)
	}
	summary.Objects++
	health, err := runtime.objects.CheckHealth(ctx, meta.Bucket, meta.Key)
	if err != nil {
		summary.Failed++
		runtime.logger.WarnContext(ctx, "check object health failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
		return nil
	}
	if health.Missing == 0 && health.Corrupted == 0 {
		return nil
	}
	summary.Missing += health.Missing
	summary.Corrupted += health.Corrupted
	summary.Unhealthy++
	if !health.Recoverable {
		summary.Unrecoverable++
		runtime.logger.WarnContext(ctx, "object shards are not recoverable",
			"bucket", meta.Bucket,
			"key", meta.Key,
			"missing", health.Missing,
			"corrupted", health.Corrupted,
			"available", health.Available,
			"total_chunks", health.TotalChunks,
		)
		return nil
	}
	throttled, err := limiter.Wait(ctx)
	if err != nil {
		return err
	}
	if throttled {
		summary.Throttled++
	}
	summary.RepairAttempts++
	repairedShards, err := repairObjectWithRetry(ctx, runtime, meta, summary)
	if err != nil {
		summary.Failed++
		runtime.logger.WarnContext(ctx, "repair object failed", "bucket", meta.Bucket, "key", meta.Key, "error", err)
		return nil
	}
	if repairedShards > 0 {
		summary.RepairedObjects++
		summary.RepairedShards += repairedShards
	}
	return nil
}
