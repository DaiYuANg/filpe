// Package http wires MaxIO's HTTP server runtime.
package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/httpx"
	httpxfiber "github.com/arcgolabs/httpx/adapter/fiber"
	fiberapp "github.com/gofiber/fiber/v2"
	fiberadapter "github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/lyonbrown4d/maxio/internal/config"
	"github.com/lyonbrown4d/maxio/internal/handler"
)

type httpRuntime struct {
	cfg     config.Config
	logger  *slog.Logger
	server  httpx.ServerRuntime
	app     *fiberapp.App
	service *handler.Service
}

func Module() dix.Module {
	return dix.NewModule(
		"http",
		dix.WithModuleProviders(
			dix.Provider0(func() *fiberapp.App {
				return fiberapp.New()
			}),
			dix.Provider2(func(
				logger *slog.Logger,
				app *fiberapp.App,
			) httpx.ServerRuntime {
				return httpx.New(
					httpx.WithAdapter(httpxfiber.New(app)),
					httpx.WithLogger(logger),
				)
			}),
			dix.Provider5(func(cfg config.Config, logger *slog.Logger, server httpx.ServerRuntime, app *fiberapp.App, service *handler.Service) *httpRuntime {
				return &httpRuntime{
					cfg:     cfg,
					logger:  logger,
					server:  server,
					app:     app,
					service: service,
				}
			}),
		),
		dix.Hooks(
			dix.OnStart(func(ctx context.Context, rt *httpRuntime) error {
				router := stdhttp.NewServeMux()
				rt.service.RegisterHTTP(router)
				rt.app.All("/*", fiberadapter.HTTPHandler(router))

				go func() {
					if err := rt.server.ListenAndServeContext(ctx, rt.cfg.HTTPAddress); err != nil {
						if !errors.Is(err, stdhttp.ErrServerClosed) {
							rt.logger.ErrorContext(ctx, "http server stopped", "error", err)
						}
					}
				}()

				rt.logger.InfoContext(ctx, "http server started", "addr", rt.cfg.HTTPAddress)
				return nil
			}),
			dix.OnStop(func(_ context.Context, rt *httpRuntime) error {
				rt.logger.Info("http server stopping")
				if err := rt.server.Shutdown(); err != nil {
					return fmt.Errorf("shutdown http server: %w", err)
				}
				rt.logger.Info("http server stopped")
				return nil
			}),
		),
	)
}
