package object

import (
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/eventx"
	"github.com/lyonbrown4d/maxio/internal/index"
	"github.com/lyonbrown4d/maxio/internal/store"
)

func Module() dix.Module {
	return dix.NewModule(
		"object",
		dix.WithModuleProviders(
			dix.Provider4(func(
				storage *store.Store,
				search *index.SearchEngine,
				bus eventx.BusRuntime,
				logger *slog.Logger,
			) *Service {
				return NewService(storage, search, bus, logger)
			}),
		),
	)
}
