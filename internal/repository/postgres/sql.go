package postgres

import (
	"context"

	"github.com/Masterminds/squirrel"

	infrapg "github.com/arrowwhi/infrastructure/connections/postgres"
)

// builder всегда генерирует плейсхолдеры $1, $2 — формат PostgreSQL.
var builder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

// queryName помечает запросы метода стабильным именем: оно становится меткой
// db.query.name в метриках и полем в логе медленных запросов коннектора.
// Вложенный вызов перезаписывает имя своим — в метрики попадает самый точный.
func queryName(ctx context.Context, name string) context.Context {
	return infrapg.WithQueryName(ctx, name)
}
