// Package handler provides MaxIO HTTP route handlers.
package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/eventx"
	"github.com/arcgolabs/logx"
	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/object"
)

func Module() dix.Module {
	return dix.NewModule(
		"infra",
		dix.WithModuleProviders(
			dix.Provider1(newLogger),
			dix.Provider1(func(logger *slog.Logger) eventx.BusRuntime {
				return eventx.New(eventx.WithMiddleware(busMiddleware(logger)))
			}),
			dix.Provider3(NewService),
		),
		dix.Hooks(
			dix.OnStart2(func(_ context.Context, bus eventx.BusRuntime, logger *slog.Logger) error {
				_, err := eventx.Subscribe(bus, func(_ context.Context, event object.ObjectEvent) error {
					logger.Info("object event",
						"event", event.Payload["event"],
						"bucket", event.Payload["bucket"],
						"key", event.Payload["key"],
					)
					return nil
				})
				if err != nil {
					return fmt.Errorf("subscribe object event failed: %w", err)
				}
				return nil
			}),
			dix.OnStop(func(_ context.Context, bus eventx.BusRuntime) error {
				if err := bus.Close(); err != nil {
					return fmt.Errorf("close event bus: %w", err)
				}
				return nil
			}),
		),
	)
}

func busMiddleware(logger *slog.Logger) eventx.Middleware {
	return func(handlerFn eventx.HandlerFunc) eventx.HandlerFunc {
		return func(ctx context.Context, event eventx.Event) error {
			if err := handlerFn(ctx, event); err != nil {
				logger.ErrorContext(ctx, "event bus handler error", "event", event.Name(), "error", err)
				return err
			}
			return nil
		}
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level, err := logx.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = slog.LevelInfo
	}

	logger, err := logx.New(
		logx.WithLevel(level),
		logx.WithCaller(true),
		logx.WithGlobalLogger(),
	)
	if err == nil {
		return logger
	}
	return slog.Default()
}
