package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lyonbrown4d/maxio"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := maxio.Run(ctx); err != nil {
		slog.Error("maxio failed", "error", err)
	}
}
