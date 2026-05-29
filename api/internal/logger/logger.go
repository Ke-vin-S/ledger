package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey struct{}

var global *zap.Logger

// Init builds the application logger. Call once at startup.
// In local env: colored console output. Otherwise: JSON to stdout (CloudWatch / Datadog / Loki ready).
func Init(env, logLevel string) *zap.Logger {
	level := zapcore.InfoLevel
	_ = level.UnmarshalText([]byte(logLevel))

	var cfg zap.Config
	if env == "local" {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.Level = zap.NewAtomicLevelAt(level)

	log, err := cfg.Build(zap.Fields(
		zap.String("service", "splitleger-api"),
		zap.String("env", env),
	))
	if err != nil {
		log = zap.NewNop()
	}

	global = log
	return log
}

// WithContext stores a logger in ctx (used by the request logging middleware).
func WithContext(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, log)
}

// FromContext returns the logger stored by WithContext, falling back to the
// global logger initialised by Init, then a no-op logger.
func FromContext(ctx context.Context) *zap.Logger {
	if log, ok := ctx.Value(contextKey{}).(*zap.Logger); ok && log != nil {
		return log
	}
	if global != nil {
		return global
	}
	return zap.NewNop()
}
