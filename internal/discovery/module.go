package discovery

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/lyonbrown4d/maxio/internal/config"
)

func Module() dix.Module {
	return dix.NewModule(
		"discovery",
		dix.WithModuleProviders(
			dix.Provider2(func(cfg config.Config, logger *slog.Logger) *Runtime {
				return NewRuntime(cfg, logger)
			}),
		),
		dix.Hooks(
			dix.OnStart(func(ctx context.Context, rt *Runtime) error {
				if err := rt.Start(ctx); err != nil {
					return fmt.Errorf("start discovery: %w", err)
				}
				return nil
			}),
			dix.OnStop(func(ctx context.Context, rt *Runtime) error {
				if err := rt.Stop(ctx); err != nil {
					return fmt.Errorf("stop discovery: %w", err)
				}
				return nil
			}),
		),
	)
}
