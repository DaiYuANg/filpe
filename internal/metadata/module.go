package metadata

import (
	"context"

	"github.com/arcgolabs/dix"
	raftx "github.com/lyonbrown4d/maxio/internal/raft"
)

func Module() dix.Module {
	return dix.NewModule("metadata",
		dix.WithModuleProviders(
			dix.ProviderErr1(func(runtime *raftx.Runtime) (MetadataStore, error) {
				return NewRaftMetadata(runtime)
			}),
		),
		dix.Hooks(
			dix.OnStop(func(_ context.Context, store MetadataStore) error {
				if closer, ok := store.(interface{ Close() error }); ok {
					return closer.Close()
				}
				return nil
			}),
		),
	)
}
