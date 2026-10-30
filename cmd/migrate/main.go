package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/arrowwhi/kinolist/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

// Миграции живут отдельным бинарём, а не внутри app: приложение схему не
// меняет, а контейнер прогоняет этот шаг перед стартом.
func run() error {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return errors.New("POSTGRES_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()

	// При rolling-деплое старый и новый контейнер могут стартовать почти
	// одновременно — advisory-lock не даст накатить миграции дважды.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("locker: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}

	for _, r := range results {
		fmt.Printf("applied %s (%s)\n", r.Source.Path, r.Duration)
	}
	if len(results) == 0 {
		fmt.Println("schema is up to date")
	}

	return nil
}
