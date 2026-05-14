// Package scheduler provides the cluster-aware background job scheduler.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"
	gocron "github.com/go-co-op/gocron/v2"
	"github.com/lyonbrown4d/maxio/internal/raft"
)

// Runtime wraps gocron with MaxIO lifecycle and Raft-backed leader election.
type Runtime struct {
	scheduler gocron.Scheduler
	elector   *raftElector
	logger    *slog.Logger
}

func Module() dix.Module {
	return dix.NewModule(
		"scheduler",
		dix.WithModuleProviders(
			dix.ProviderErr2(newRuntime),
		),
		dix.Hooks(
			dix.OnStart(startRuntime),
			dix.OnStop(stopRuntime),
		),
	)
}

func newRuntime(logger *slog.Logger, raftRuntime *raft.Runtime) (*Runtime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	elector := &raftElector{runtime: raftRuntime}
	scheduler, err := gocron.NewScheduler(
		gocron.WithLogger(slogCronLogger{logger: logger.With("component", "gocron")}),
		gocron.WithDistributedElector(elector),
		gocron.WithGlobalJobOptions(
			gocron.WithSingletonMode(gocron.LimitModeReschedule),
			gocron.WithIntervalFromCompletion(),
		),
		gocron.WithStopTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("create scheduler: %w", err)
	}
	return &Runtime{
		scheduler: scheduler,
		elector:   elector,
		logger:    logger,
	}, nil
}

func startRuntime(ctx context.Context, runtime *Runtime) error {
	if runtime == nil || runtime.scheduler == nil {
		return nil
	}
	runtime.scheduler.Start()
	if runtime.logger != nil {
		runtime.logger.InfoContext(ctx, "scheduler started")
	}
	return nil
}

func stopRuntime(ctx context.Context, runtime *Runtime) error {
	if runtime == nil || runtime.scheduler == nil {
		return nil
	}
	if err := runtime.scheduler.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("shutdown scheduler: %w", err)
	}
	if runtime.logger != nil {
		runtime.logger.InfoContext(ctx, "scheduler stopped")
	}
	return nil
}

func (runtime *Runtime) NewJob(definition gocron.JobDefinition, task gocron.Task, options ...gocron.JobOption) (gocron.Job, error) {
	if runtime == nil || runtime.scheduler == nil {
		return nil, errors.New("scheduler unavailable")
	}
	job, err := runtime.scheduler.NewJob(definition, task, options...)
	if err != nil {
		return nil, fmt.Errorf("create scheduled job: %w", err)
	}
	return job, nil
}

func (runtime *Runtime) RequireLeader(ctx context.Context) error {
	if runtime == nil || runtime.elector == nil {
		return raft.ErrLeaderUnavailable
	}
	return runtime.elector.IsLeader(ctx)
}

type raftElector struct {
	runtime *raft.Runtime
}

func (elector *raftElector) IsLeader(ctx context.Context) error {
	if elector == nil || elector.runtime == nil {
		return raft.ErrLeaderUnavailable
	}
	if err := elector.runtime.AssertLeader(ctx); err != nil {
		return fmt.Errorf("check raft scheduler leadership: %w", err)
	}
	return nil
}

type slogCronLogger struct {
	logger *slog.Logger
}

func (logger slogCronLogger) Debug(message string, args ...any) {
	logger.log().Debug(message, args...)
}

func (logger slogCronLogger) Error(message string, args ...any) {
	logger.log().Error(message, args...)
}

func (logger slogCronLogger) Info(message string, args ...any) {
	logger.log().Info(message, args...)
}

func (logger slogCronLogger) Warn(message string, args ...any) {
	logger.log().Warn(message, args...)
}

func (logger slogCronLogger) log() *slog.Logger {
	if logger.logger == nil {
		return slog.Default()
	}
	return logger.logger
}
