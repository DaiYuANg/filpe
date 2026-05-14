// Package object exposes MaxIO's public object service API.
package object

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"object",
		dix.WithModuleProviders(
			dix.Provider4(NewService),
		),
	)
}
