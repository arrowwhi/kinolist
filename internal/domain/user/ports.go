package user

import "context"

type Repository interface {
	// EnsureExists заводит пользователя при первом обращении и возвращает
	// актуальную запись — отдельного шага регистрации в боте нет.
	EnsureExists(ctx context.Context, u User) (User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
}
