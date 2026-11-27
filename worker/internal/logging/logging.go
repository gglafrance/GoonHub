// Package logging sets up the worker's structured logs: console and a rotated file.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Options configure New.
type Options struct {
	Level      string // debug, info, warn, error
	Console    io.Writer
	ConsoleFmt string // text or json
	File       string // "" or "-" = no file
	FileFmt    string // json or text
	MaxSizeMB  int
	MaxBackups int
}

// New builds a logger writing to the console and (optionally) a rotated file. The closer
// flushes and closes the file.
func New(o Options) (*slog.Logger, io.Closer, error) {
	level, err := ParseLevel(o.Level)
	if err != nil {
		return nil, nil, err
	}
	hopts := &slog.HandlerOptions{Level: level}
	var handlers []slog.Handler
	if o.Console != nil {
		handlers = append(handlers, newHandler(o.Console, o.ConsoleFmt, hopts))
	}
	var closer io.Closer = nopCloser{}
	if o.File != "" && o.File != "-" {
		if err := os.MkdirAll(filepath.Dir(o.File), 0o755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		lj := &lumberjack.Logger{Filename: o.File, MaxSize: o.MaxSizeMB, MaxBackups: o.MaxBackups}
		handlers = append(handlers, newHandler(lj, o.FileFmt, hopts))
		closer = lj
	}
	if len(handlers) == 0 {
		return slog.New(slog.NewTextHandler(io.Discard, nil)), closer, nil
	}
	if len(handlers) == 1 {
		return slog.New(handlers[0]), closer, nil
	}
	return slog.New(fanout(handlers)), closer, nil
}

// ParseLevel parses debug/info/warn/error.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("unknown log level %q (debug, info, warn, error)", s)
}

func newHandler(w io.Writer, format string, o *slog.HandlerOptions) slog.Handler {
	if strings.EqualFold(format, "json") {
		return slog.NewJSONHandler(w, o)
	}
	return slog.NewTextHandler(w, o)
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// fanout sends every record to several handlers.
type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}
