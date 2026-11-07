package postgres

import (
	"context"
	"fmt"

	infrapg "github.com/arrowwhi/infrastructure/connections/postgres"

	"github.com/arrowwhi/kinolist/internal/config"
)

// serviceName — имя meter'а и метка в метриках запросов к БД.
const serviceName = "kinolist"

// NewPool поднимает пул коннектора инфраструктуры и сразу проверяет соединение:
// падать на старте понятнее, чем отдавать 500 на первом же запросе.
//
// Инструментация включена всегда: спан на каждый запрос (виден в трейсе HTTP-запроса,
// задачи воркера или обновления бота), метрики длительности запросов и состояния
// пула, лог медленных запросов. Без настроенного трейсинга спаны no-op и почти
// ничего не стоят.
func NewPool(ctx context.Context, cfg config.Postgres) (*infrapg.Pool, error) {
	poolCfg := infrapg.DefaultConfig()
	poolCfg.DSN = cfg.DSN
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.ApplicationName = serviceName
	poolCfg.TracingEnabled = true
	poolCfg.MetricsEnabled = true
	poolCfg.SlowQueryLog = cfg.SlowQueryThreshold > 0
	poolCfg.SlowQueryThreshold = cfg.SlowQueryThreshold

	pool, err := infrapg.New(ctx, poolCfg, infrapg.WithServiceName(serviceName))
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return pool, nil
}
