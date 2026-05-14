package metadata

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/arcgolabs/dix"
	"github.com/lyonbrown4d/maxio/internal/config"
)

func Module() dix.Module {
	return dix.NewModule(
		"metadata",
		dix.WithModuleProviders(
			dix.ProviderErr1(func(cfg config.Config) (MetadataStore, error) {
				if cfg.DataDir == "" {
					return nil, errors.New("data dir is empty")
				}
				path := filepath.Clean(filepath.Join(cfg.DataDir, "metadata"))
				return NewBadgerMetadata(path)
			}),
		),
		dix.Hooks(
			dix.OnStop(func(_ context.Context, store MetadataStore) error {
				if store == nil {
					return nil
				}
				if closer, ok := store.(interface{ Close() error }); ok {
					return closer.Close()
				}
				return nil
			}),
		),
	)
}
