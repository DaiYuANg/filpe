// Package index provides an object metadata search module backed by Bleve.
package index

import (
	"context"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"index",
		dix.WithModuleProviders(
			dix.ProviderErr2(NewSearchEngine),
		),
		dix.Hooks(
			dix.OnStop(func(_ context.Context, search *SearchEngine) error {
				return search.Close()
			}),
		),
	)
}
