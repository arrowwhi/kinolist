// Package logger — тонкая обёртка над логгером инфраструктуры
// (github.com/arrowwhi/infrastructure/observability/logger).
//
// Логгер глобальный и берёт поля из контекста: trace_id/span_id активного спана
// добавляет инфраструктура, а поля запроса или задачи (request_id, job_id, user_id)
// кладутся в контекст через With и попадают во все записи ниже по стеку — нижним
// слоям не нужно получать логгер отдельным параметром.
package logger

import (
	"context"

	"github.com/rs/zerolog"

	infra "github.com/arrowwhi/infrastructure/observability/logger"
)

type fieldsKey struct{}

type field struct {
	value any
	key   string
}

// Init настраивает глобальный логгер инфраструктуры и подключает поля из контекста.
// Вызывается один раз на старте (это делает telegram/app.New — здесь только извлечение полей).
func Init() {
	infra.SetContextExtractor(func(evt *zerolog.Event, ctx context.Context) *zerolog.Event {
		fields, _ := ctx.Value(fieldsKey{}).([]field)
		for _, f := range fields {
			evt = evt.Interface(f.key, f.value)
		}

		return evt
	})
}

// With возвращает контекст, записи из которого будут содержать поле key=value.
func With(ctx context.Context, key string, value any) context.Context {
	prev, _ := ctx.Value(fieldsKey{}).([]field)

	fields := make([]field, 0, len(prev)+1)
	fields = append(fields, prev...)
	fields = append(fields, field{key: key, value: value})

	return context.WithValue(ctx, fieldsKey{}, fields)
}

func Debug(ctx context.Context) *zerolog.Event { return infra.Debug(ctx) }
func Info(ctx context.Context) *zerolog.Event  { return infra.Info(ctx) }
func Warn(ctx context.Context) *zerolog.Event  { return infra.Warn(ctx) }
func Error(ctx context.Context) *zerolog.Event { return infra.Error(ctx) }

// WarnDetached — warn-запись без контекста, для мест, где контекста нет и не будет
// (например, ошибка записи ответа после отправки заголовков).
func WarnDetached() *zerolog.Event {
	l := infra.With().Logger()
	return l.Warn()
}
