package postgres

import "github.com/Masterminds/squirrel"

// builder всегда генерирует плейсхолдеры $1, $2 — формат PostgreSQL.
var builder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
