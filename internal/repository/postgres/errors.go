package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Коды PostgreSQL, которые репозитории переводят в доменные ошибки.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	return hasCode(err, codeUniqueViolation)
}

func hasCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}

// errNoExternalID означает попытку сохранить фильм без единого внешнего
// идентификатора — такую карточку невозможно ни найти, ни дедуплицировать.
var errNoExternalID = errors.New("movie has neither tmdb_id nor kinopoisk_id")
