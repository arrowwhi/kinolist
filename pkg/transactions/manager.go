// Package transactions даёт репозиториям единый способ работать как внутри
// транзакции, так и вне её: usecase открывает транзакцию, а репозиторий
// сам достаёт её из контекста и ничего не знает о вызывающем.
package transactions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// DB покрывает и *pgxpool.Pool, и pgx.Tx — репозитории принимают только его
// и потому одинаково работают внутри транзакции и вне неё.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// BeginTransaction открывает транзакцию и кладёт её в контекст.
// Возвращённый контекст обязан быть передан в FinishTransaction.
func (m *Manager) BeginTransaction(ctx context.Context) (context.Context, error) {
	if _, ok := txFrom(ctx); ok {
		// Вложенный вызов переиспользует внешнюю транзакцию: его
		// FinishTransaction станет no-op, коммитит владелец.
		return ctx, nil
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return ctx, fmt.Errorf("begin transaction: %w", err)
	}

	return context.WithValue(ctx, ctxKey{}, &holder{tx: tx, owner: true}), nil
}

// FinishTransaction коммитит транзакцию при *errPtr == nil и откатывает иначе.
// Вызывается через defer: defer m.FinishTransaction(ctx, &err).
func (m *Manager) FinishTransaction(ctx context.Context, errPtr *error) {
	h, ok := txFrom(ctx)
	if !ok || !h.owner {
		return
	}

	if *errPtr != nil {
		if rbErr := h.tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			*errPtr = fmt.Errorf("%w (rollback failed: %w)", *errPtr, rbErr)
		}
		return
	}

	if cmErr := h.tx.Commit(ctx); cmErr != nil {
		*errPtr = fmt.Errorf("commit transaction: %w", cmErr)
	}
}

// Querier отдаёт транзакцию из контекста, если она есть, иначе пул.
func (m *Manager) Querier(ctx context.Context) DB {
	if h, ok := txFrom(ctx); ok {
		return h.tx
	}
	return m.pool
}

func (m *Manager) Pool() *pgxpool.Pool { return m.pool }

type holder struct {
	tx    pgx.Tx
	owner bool
}

func txFrom(ctx context.Context) (*holder, bool) {
	h, ok := ctx.Value(ctxKey{}).(*holder)
	return h, ok && h != nil
}
