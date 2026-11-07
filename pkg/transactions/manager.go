// Package transactions даёт репозиториям единый способ работать как внутри
// транзакции, так и вне её: usecase открывает транзакцию, а репозиторий
// сам достаёт её из контекста и ничего не знает о вызывающем.
//
// Транзакции открывает пул коннектора инфраструктуры
// (github.com/arrowwhi/infrastructure/connections/postgres): он сам кладёт
// транзакцию в контекст и направляет в неё запросы, поэтому Querier может
// всегда отдавать пул. Менеджер добавляет поверх него семантику «владельца»:
// вложенный BeginTransaction переиспользует внешнюю транзакцию, а коммитит
// только тот, кто её открыл.
package transactions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	infrapg "github.com/arrowwhi/infrastructure/connections/postgres"
)

type ctxKey struct{}

// DB — то, чем пользуются репозитории. Пул коннектора сам выполняет запрос в
// транзакции из контекста, если она там есть.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Manager struct {
	pool *infrapg.Pool
}

func NewManager(pool *infrapg.Pool) *Manager {
	return &Manager{pool: pool}
}

// BeginTransaction открывает транзакцию и кладёт её в контекст.
// Возвращённый контекст обязан быть передан в FinishTransaction.
func (m *Manager) BeginTransaction(ctx context.Context) (context.Context, error) {
	if _, ok := ctx.Value(ctxKey{}).(*holder); ok {
		// Вложенный вызов переиспользует внешнюю транзакцию, но без права
		// владельца: его FinishTransaction станет no-op, коммитит внешний.
		return context.WithValue(ctx, ctxKey{}, &holder{}), nil
	}

	txCtx, tx, err := m.pool.Begin(ctx)
	if err != nil {
		return ctx, fmt.Errorf("begin transaction: %w", err)
	}

	return context.WithValue(txCtx, ctxKey{}, &holder{tx: tx}), nil
}

// FinishTransaction коммитит транзакцию при *errPtr == nil и откатывает иначе.
// Вызывается через defer: defer m.FinishTransaction(ctx, &err).
func (m *Manager) FinishTransaction(ctx context.Context, errPtr *error) {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	if !ok || h.tx == nil {
		return
	}

	if *errPtr != nil {
		// Откат — в контексте без отмены: исходный мог уже истечь, и тогда
		// ROLLBACK не дошёл бы до базы, а соединение вернулось бы в пул грязным.
		if rbErr := h.tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil {
			*errPtr = fmt.Errorf("%w (rollback failed: %w)", *errPtr, rbErr)
		}

		return
	}

	if cmErr := h.tx.Commit(ctx); cmErr != nil {
		*errPtr = fmt.Errorf("commit transaction: %w", cmErr)
	}
}

// Querier возвращает пул: запросы с контекстом транзакции он сам направляет в неё.
func (m *Manager) Querier(context.Context) DB {
	return m.pool
}

// Pool возвращает пул коннектора.
func (m *Manager) Pool() *infrapg.Pool { return m.pool }

// holder отмечает контекст с транзакцией. tx == nil — вложенный вызов без права владельца.
type holder struct {
	tx *infrapg.Tx
}
