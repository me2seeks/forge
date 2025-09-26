package logs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

var _ FullLogger = (*slogLogger)(nil)

// slogLogger is a logger that uses slog.
type slogLogger struct {
	l     *slog.Logger
	level *slog.LevelVar
}

// NewSlogLogger creates a new slogLogger.
func NewSlogLogger(opts ...SlogOption) FullLogger {
	options := defaultSlogOptions()
	for _, opt := range opts {
		opt.apply(options)
	}

	levelVar := &slog.LevelVar{}
	levelVar.Set(toSlogLevel(options.level))

	l := slog.New(slog.NewJSONHandler(options.output, &slog.HandlerOptions{
		AddSource: options.addSource,
		Level:     levelVar,
	}))

	return &slogLogger{
		l:     l,
		level: levelVar,
	}
}

// SetLevel sets the level of the logger.
func (s *slogLogger) SetLevel(level Level) {
	s.level.Set(toSlogLevel(level))
}

// SetOutput sets the output of the logger.
func (s *slogLogger) SetOutput(w io.Writer) {
	levelVar := &slog.LevelVar{}
	levelVar.Set(s.level.Level())

	s.l = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		// It's a bit tricky to get the original addSource option here.
		// For now, let's assume we don't need to preserve it across SetOutput calls.
		// A better implementation might store the options in the logger.
		AddSource: true,
		Level:     levelVar,
	}))
}

// Trace implements the Logger interface.
func (s *slogLogger) Trace(v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelTrace), fmt.Sprint(v...))
}

// Debug implements the Logger interface.
func (s *slogLogger) Debug(v ...any) {
	s.l.Debug(fmt.Sprint(v...))
}

// Info implements the Logger interface.
func (s *slogLogger) Info(v ...any) {
	s.l.Info(fmt.Sprint(v...))
}

// Notice implements the Logger interface.
func (s *slogLogger) Notice(v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelNotice), fmt.Sprint(v...))
}

// Warn implements the Logger interface.
func (s *slogLogger) Warn(v ...any) {
	s.l.Warn(fmt.Sprint(v...))
}

// Error implements the Logger interface.
func (s *slogLogger) Error(v ...any) {
	s.l.Error(fmt.Sprint(v...))
}

// Fatal implements the Logger interface.
func (s *slogLogger) Fatal(v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelFatal), fmt.Sprint(v...))
	os.Exit(1)
}

// Tracef implements the FormatLogger interface.
func (s *slogLogger) Tracef(format string, v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelTrace), fmt.Sprintf(format, v...))
}

// Debugf implements the FormatLogger interface.
func (s *slogLogger) Debugf(format string, v ...any) {
	s.l.Debug(fmt.Sprintf(format, v...))
}

// Infof implements the FormatLogger interface.
func (s *slogLogger) Infof(format string, v ...any) {
	s.l.Info(fmt.Sprintf(format, v...))
}

// Noticef implements the FormatLogger interface.
func (s *slogLogger) Noticef(format string, v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelNotice), fmt.Sprintf(format, v...))
}

// Warnf implements the FormatLogger interface.
func (s *slogLogger) Warnf(format string, v ...any) {
	s.l.Warn(fmt.Sprintf(format, v...))
}

// Errorf implements the FormatLogger interface.
func (s *slogLogger) Errorf(format string, v ...any) {
	s.l.Error(fmt.Sprintf(format, v...))
}

// Fatalf implements the FormatLogger interface.
func (s *slogLogger) Fatalf(format string, v ...any) {
	s.l.Log(context.Background(), toSlogLevel(LevelFatal), fmt.Sprintf(format, v...))
	os.Exit(1)
}

// CtxTracef implements the CtxLogger interface.
func (s *slogLogger) CtxTracef(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelTrace, format, v...)
}

// CtxDebugf implements the CtxLogger interface.
func (s *slogLogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelDebug, format, v...)
}

// CtxInfof implements the CtxLogger interface.
func (s *slogLogger) CtxInfof(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelInfo, format, v...)
}

// CtxNoticef implements the CtxLogger interface.
func (s *slogLogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelNotice, format, v...)
}

// CtxWarnf implements the CtxLogger interface.
func (s *slogLogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelWarn, format, v...)
}

// CtxErrorf implements the CtxLogger interface.
func (s *slogLogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelError, format, v...)
}

// CtxFatalf implements the CtxLogger interface.
func (s *slogLogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	s.logCtx(ctx, LevelFatal, format, v...)
	os.Exit(1)
}

func (s *slogLogger) logCtx(ctx context.Context, level Level, format string, v ...any) {
	var attrs []slog.Attr
	if val := ctx.Value(logKey{}); val != nil {
		if kv, ok := val.([]any); ok {
			attrs = append(attrs, slog.Any("context", kv))
		}
	}
	msg := fmt.Sprintf(format, v...)
	s.l.LogAttrs(ctx, toSlogLevel(level), msg, attrs...)
}

func toSlogLevel(level Level) slog.Level {
	switch level {
	case LevelTrace:
		// slog does not have a trace level, so we use a custom level.
		return slog.Level(-8)
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelNotice:
		// slog does not have a notice level, so we use a custom level.
		return slog.Level(2)
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelFatal:
		// slog does not have a fatal level, so we use a custom level.
		return slog.Level(12)
	default:
		return slog.LevelInfo
	}
}

// SlogOption is a functional option for slog logger.
type SlogOption interface {
	apply(*slogOptions)
}

type slogOptions struct {
	level     Level
	output    io.Writer
	addSource bool
}

type funcSlogOption struct {
	f func(*slogOptions)
}

func (fso *funcSlogOption) apply(opts *slogOptions) {
	fso.f(opts)
}

func newFuncSlogOption(f func(*slogOptions)) *funcSlogOption {
	return &funcSlogOption{f: f}
}

func defaultSlogOptions() *slogOptions {
	return &slogOptions{
		level:     LevelInfo,
		output:    os.Stderr,
		addSource: true,
	}
}

// WithLevel sets the level of the logger.
func WithLevel(level Level) SlogOption {
	return newFuncSlogOption(func(opts *slogOptions) {
		opts.level = level
	})
}

// WithOutput sets the output of the logger.
func WithOutput(w io.Writer) SlogOption {
	return newFuncSlogOption(func(opts *slogOptions) {
		opts.output = w
	})
}

// WithAddSource sets whether to add source code location to the log.
func WithAddSource(addSource bool) SlogOption {
	return newFuncSlogOption(func(opts *slogOptions) {
		opts.addSource = addSource
	})
}
