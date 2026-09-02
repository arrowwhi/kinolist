package postgres

import (
	"context"
	"fmt"

	"github.com/arrowwhi/kinolist/internal/domain/user"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

type UserRepository struct {
	tx *transactions.Manager
}

func NewUserRepository(tx *transactions.Manager) *UserRepository {
	return &UserRepository{tx: tx}
}

const userColumns = "id, telegram_id, coalesce(username, ''), coalesce(first_name, ''), language_code, created_at, updated_at"

// EnsureExists заводит пользователя при первом обращении и обновляет
// профиль при последующих: имя и username в Telegram меняются.
func (r *UserRepository) EnsureExists(ctx context.Context, u user.User) (user.User, error) {
	const q = `
		INSERT INTO users (telegram_id, username, first_name, language_code)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		SET username      = EXCLUDED.username,
		    first_name    = EXCLUDED.first_name,
		    language_code = EXCLUDED.language_code,
		    updated_at    = now()
		RETURNING ` + userColumns

	lang := u.LanguageCode
	if lang == "" {
		lang = "ru"
	}

	row := r.tx.Querier(ctx).QueryRow(ctx, q, u.TelegramID, u.Username, u.FirstName, lang)

	out, err := scanUser(row)
	if err != nil {
		return user.User{}, fmt.Errorf("ensure user exists: %w", err)
	}

	return out, nil
}

func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (user.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE telegram_id = $1`

	out, err := scanUser(r.tx.Querier(ctx).QueryRow(ctx, q, telegramID))
	if err != nil {
		if isNoRows(err) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("get user by telegram id: %w", err)
	}

	return out, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (user.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`

	out, err := scanUser(r.tx.Querier(ctx).QueryRow(ctx, q, id))
	if err != nil {
		if isNoRows(err) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("get user by id: %w", err)
	}

	return out, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (user.User, error) {
	var u user.User
	err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LanguageCode, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}
