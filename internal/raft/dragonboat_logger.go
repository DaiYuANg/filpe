// Package raft integrates Dragonboat with MaxIO metadata replication.
package raft

import (
	"fmt"
	"log/slog"
	"sync"

	dlog "github.com/lni/dragonboat/v4/logger"
)

type dragonboatSlogLogger struct {
	base *slog.Logger
	pkg  string
}

var configureDragonboatLoggerOnce sync.Once

func (l *dragonboatSlogLogger) SetLevel(_ dlog.LogLevel) {}

func (l *dragonboatSlogLogger) Debugf(format string, args ...any) {
	if l.base == nil {
		return
	}
	l.base.Debug(fmt.Sprintf(format, args...), "component", "dragonboat", "pkg", l.pkg)
}

func (l *dragonboatSlogLogger) Infof(format string, args ...any) {
	if l.base == nil {
		return
	}
	l.base.Info(fmt.Sprintf(format, args...), "component", "dragonboat", "pkg", l.pkg)
}

func (l *dragonboatSlogLogger) Warningf(format string, args ...any) {
	if l.base == nil {
		return
	}
	l.base.Warn(fmt.Sprintf(format, args...), "component", "dragonboat", "pkg", l.pkg)
}

func (l *dragonboatSlogLogger) Errorf(format string, args ...any) {
	if l.base == nil {
		return
	}
	l.base.Error(fmt.Sprintf(format, args...), "component", "dragonboat", "pkg", l.pkg)
}

func (l *dragonboatSlogLogger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if l.base == nil {
		panic(msg)
	}
	l.base.Error(msg, "component", "dragonboat", "pkg", l.pkg)
	panic(msg)
}

func configureDragonboatLogger(baseLogger *slog.Logger) {
	configureDragonboatLoggerOnce.Do(func() {
		dlog.SetLoggerFactory(func(pkg string) dlog.ILogger {
			logger := baseLogger
			if logger == nil {
				logger = slog.Default()
			}
			return &dragonboatSlogLogger{
				base: logger,
				pkg:  pkg,
			}
		})
	})
}
