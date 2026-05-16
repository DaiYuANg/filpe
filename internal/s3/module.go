// Package s3 provides MaxIO's S3-compatible HTTP endpoint.
package s3

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"s3",
		dix.WithModuleProviders(
			dix.Provider3(NewService),
			dix.Provider1(NewEndpoint),
		),
	)
}
