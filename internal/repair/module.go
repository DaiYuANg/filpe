// Package repair schedules and runs background object shard repair.
package repair

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/arcgolabs/dix"
	gocron "github.com/go-co-op/gocron/v2"
	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/scheduler"
	"github.com/lyonbrown4d/maxio/object"
)

const repairJobName = "maxio.object.repair"
const repairRunTriggerScheduled = "scheduled"
const repairRunTriggerManual = "manual"

// Summary reports one repair scan result.
type Summary struct {
	RunID           string  `json:"run_id"`
	Buckets         int     `json:"buckets"`
	Objects         int     `json:"objects"`
	Unhealthy       int     `json:"unhealthy"`
	Missing         int     `json:"missing"`
	Corrupted       int     `json:"corrupted"`
	Scrubbed        int     `json:"scrubbed"`
	ScrubFailed     int     `json:"scrub_failed"`
	ChecksumFailed  int     `json:"checksum_failed"`
	RepairAttempts  int     `json:"repair_attempts"`
	RepairRetries   int     `json:"repair_retries"`
	RetryDelayMs    int64   `json:"retry_delay_ms"`
	Throttled       int     `json:"throttled"`
	ThrottleWaitMs  int64   `json:"throttle_wait_ms"`
	RepairedObjects int     `json:"repaired_objects"`
	RepairedShards  int     `json:"repaired_shards"`
	Unrecoverable   int     `json:"unrecoverable"`
	Failed          int     `json:"failed"`
	Limited         bool    `json:"limited"`
	Issues          []Issue `json:"issues,omitempty"`
}

var ErrRepairAlreadyRunning = errors.New("repair run already in progress")

// Runtime owns the scheduled repair job.
type Runtime struct {
	cfg         config.Config
	objects     *object.Service
	scheduler   *scheduler.Runtime
	logger      *slog.Logger
	mu          sync.RWMutex
	status      Status
	history     []RunRecord
	issues      map[string][]Issue
	lastTrigger string
	nextRunID   atomic.Uint64
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
		issues:    make(map[string][]Issue),
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
		runtime.setRunTrigger(repairRunTriggerScheduled)
		runtime.runScheduled(runCtx)
	}()
}

func (runtime *Runtime) runScheduled(ctx context.Context) {
	runtime.setRunTrigger(repairRunTriggerScheduled)
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

func (runtime *Runtime) setRunTrigger(trigger string) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.lastTrigger = strings.TrimSpace(trigger)
	if runtime.lastTrigger == "" {
		runtime.lastTrigger = repairRunTriggerManual
	}
}

func (runtime *Runtime) RunOnce(ctx context.Context) (Summary, error) {
	if runtime == nil {
		return Summary{}, errors.New("repair runtime unavailable")
	}
	runtime.setRunTrigger(repairRunTriggerManual)
	runID := runtime.newRunID()
	startedAt, started := runtime.tryMarkStarted(runID)
	if !started {
		return Summary{}, ErrRepairAlreadyRunning
	}

	summary, err := runtime.runOnce(ctx, runID)
	runtime.markFinished(startedAt, runID, summary, err)

	return summary, err
}
