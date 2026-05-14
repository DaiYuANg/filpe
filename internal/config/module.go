package config

import (
	"github.com/arcgolabs/configx"
	"github.com/arcgolabs/dix"
)

func Module(opts ...configx.Option) dix.Module {
	options := make([]configx.Option, 0, 1+len(opts))
	options = append(options, opts...)

	return dix.NewModule(
		"config",
		dix.WithModuleProviders(
			dix.ProviderErr0(func() (Config, error) {
				return Load(options...)
			}),
		),
	)
}
