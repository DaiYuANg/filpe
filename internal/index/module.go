// Package index provides an object metadata search module backed by Bleve.
package index

import (
	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"index",
		dix.WithModuleProviders(
			dix.Provider0(NewSearchEngine),
		),
	)
}
