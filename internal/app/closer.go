package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Closer закрывает ресурсы в порядке, обратном их инициализации: пул БД
// должен пережить транспорт, который им ещё пользуется.
type Closer struct {
	mu    sync.Mutex
	funcs []namedCloser
}

type namedCloser struct {
	name string
	fn   func(context.Context) error
}

func NewCloser() *Closer {
	return &Closer{}
}

func (c *Closer) Add(name string, fn func(context.Context) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.funcs = append(c.funcs, namedCloser{name: name, fn: fn})
}

// Close выполняет все функции в обратном порядке. Сбой одной не отменяет
// остальные: незакрытые ресурсы хуже, чем несколько ошибок в логе.
func (c *Closer) Close(ctx context.Context) error {
	c.mu.Lock()
	funcs := c.funcs
	c.funcs = nil
	c.mu.Unlock()

	var errs []error
	for i := len(funcs) - 1; i >= 0; i-- {
		if err := funcs[i].fn(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", funcs[i].name, err))
		}
	}

	return errors.Join(errs...)
}
