package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/arrowwhi/kinolist/pkg/transactions"
)

// CacheRepository — кэш ответов внешних каталогов. Живёт в Postgres, чтобы
// не тащить в проект отдельный Redis ради одной таблицы.
type CacheRepository struct {
	tx *transactions.Manager
}

func NewCacheRepository(tx *transactions.Manager) *CacheRepository {
	return &CacheRepository{tx: tx}
}

func (r *CacheRepository) Get(ctx context.Context, key string) ([]byte, bool, error) {
	const q = `SELECT payload FROM catalog_cache WHERE cache_key = $1 AND expires_at > now()`

	var payload []byte
	if err := r.tx.Querier(ctx).QueryRow(ctx, q, key).Scan(&payload); err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get cache entry: %w", err)
	}

	return payload, true, nil
}

func (r *CacheRepository) Set(ctx context.Context, key string, payload []byte, ttl time.Duration) error {
	const q = `
		INSERT INTO catalog_cache (cache_key, payload, expires_at)
		VALUES ($1, $2, now() + $3::interval)
		ON CONFLICT (cache_key) DO UPDATE
		SET payload = EXCLUDED.payload, expires_at = EXCLUDED.expires_at`

	if _, err := r.tx.Querier(ctx).Exec(ctx, q, key, payload, ttl.String()); err != nil {
		return fmt.Errorf("set cache entry: %w", err)
	}

	return nil
}

// DeleteExpired вызывается фоном: без уборки таблица растёт бесконечно.
func (r *CacheRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.tx.Querier(ctx).Exec(ctx, `DELETE FROM catalog_cache WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired cache entries: %w", err)
	}
	return tag.RowsAffected(), nil
}
