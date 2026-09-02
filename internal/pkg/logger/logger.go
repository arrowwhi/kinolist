package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey struct{}

// New собирает логгер: в проде JSON, локально — человекочитаемый текст.
func New(env, level string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", level, err)
	}

	var cfg zap.Config
	if env == "prod" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.Encoding = "console"
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	log, err := cfg.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return log, nil
}

// Into кладёт логгер в контекст, чтобы нижние слои писали с теми же полями
// (request_id, user_id), не получая логгер отдельным параметром.
func Into(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// From достаёт логгер из контекста; если его там нет — возвращает no-op,
// чтобы отсутствие логгера никогда не роняло обработчик.
func From(ctx context.Context) *zap.Logger {
	if log, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && log != nil {
		return log
	}
	return zap.NewNop()
}

func With(ctx context.Context, fields ...zap.Field) context.Context {
	return Into(ctx, From(ctx).With(fields...))
}
