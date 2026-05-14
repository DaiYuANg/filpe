package discovery

import (
	"context"
	"fmt"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"discovery",
		dix.WithModuleProviders(
			dix.Provider2(NewRuntime),
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
