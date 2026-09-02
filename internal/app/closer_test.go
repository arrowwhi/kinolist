package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arrowwhi/kinolist/internal/app"
)

func TestCloserRunsInReverseOrder(t *testing.T) {
	t.Parallel()

	var order []string
	c := app.NewCloser()

	c.Add("first", func(context.Context) error { order = append(order, "first"); return nil })
	c.Add("second", func(context.Context) error { order = append(order, "second"); return nil })
	c.Add("third", func(context.Context) error { order = append(order, "third"); return nil })

	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"third", "second", "first"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestCloserKeepsGoingAfterAFailure(t *testing.T) {
	t.Parallel()

	closed := 0
	boom := errors.New("boom")

	c := app.NewCloser()
	c.Add("db", func(context.Context) error { closed++; return nil })
	c.Add("broken", func(context.Context) error { return boom })
	c.Add("cache", func(context.Context) error { closed++; return nil })

	err := c.Close(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap boom", err)
	}
	if closed != 2 {
		t.Fatalf("closed %d resources, want 2 — a failure must not skip the rest", closed)
	}
}

func TestCloserIsIdempotent(t *testing.T) {
	t.Parallel()

	calls := 0
	c := app.NewCloser()
	c.Add("once", func(context.Context) error { calls++; return nil })

	_ = c.Close(context.Background())
	_ = c.Close(context.Background())

	if calls != 1 {
		t.Fatalf("close func called %d times, want 1", calls)
	}
}
