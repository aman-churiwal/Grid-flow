package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type contextKey string

const TraceIDKey contextKey = "trace_id"

type Logger struct {
	logger zerolog.Logger
}

func NewLogger(serviceName, logLevel, appEnv string) *Logger {
	var logger zerolog.Logger
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	if appEnv == "development" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			Level(level).
			With().
			Timestamp().
			Str("service_name", serviceName).
			Logger()
	} else {
		logger = zerolog.New(os.Stdout).
			Level(level).
			With().
			Timestamp().
			Str("service_name", serviceName).
			Logger()
	}

	return &Logger{logger: logger}
}

func (l *Logger) Info(ctx context.Context) *zerolog.Event {
	event := l.logger.Info()

	if traceId, ok := ctx.Value(TraceIDKey).(string); ok && traceId != "" {
		event = event.Str("trace_id", traceId)
	}

	return event
}

func (l *Logger) Error(ctx context.Context) *zerolog.Event {
	event := l.logger.Error()

	if traceId, ok := ctx.Value(TraceIDKey).(string); ok && traceId != "" {
		event = event.Str("trace_id", traceId)
	}

	return event
}

func (l *Logger) Warn(ctx context.Context) *zerolog.Event {
	event := l.logger.Warn()

	if traceId, ok := ctx.Value(TraceIDKey).(string); ok && traceId != "" {
		event = event.Str("trace_id", traceId)
	}

	return event
}

func (l *Logger) Debug(ctx context.Context) *zerolog.Event {
	event := l.logger.Debug()

	if traceId, ok := ctx.Value(TraceIDKey).(string); ok && traceId != "" {
		event = event.Str("trace_id", traceId)
	}

	return event
}
