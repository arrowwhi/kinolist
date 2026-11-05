// Command backfill fills missing catalogue metadata without changing user data.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/arrowwhi/kinolist/internal/config"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/repository/kinopoisk"
	"github.com/arrowwhi/kinolist/internal/repository/postgres"
	"github.com/arrowwhi/kinolist/internal/repository/tmdb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	apply := flag.Bool("apply", false, "Write metadata; without this flag only list matching IDs")
	limit := flag.Int("limit", 100, "Maximum movies per run (1-1000)")
	after := flag.Int64("after-id", 0, "Resume after this catalogue movie ID")
	flag.Parse()
	if *limit < 1 || *limit > 1000 || *after < 0 {
		return errors.New("invalid limit or after-id")
	}
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT id,coalesce(tmdb_id,0),coalesce(kinopoisk_id,0) FROM movies
 WHERE id>$1 AND (runtime_min IS NULL OR rating_provider IS NULL OR (tmdb_id IS NOT NULL AND rating_vote_count IS NULL)) ORDER BY id LIMIT $2`, *after, *limit)
	if err != nil {
		return err
	}
	type entry struct {
		id       int64
		tmdb, kp int32
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.tmdb, &e.kp); err != nil {
			rows.Close()
			return err
		}
		entries = append(entries, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	primary := tmdb.New(cfg.TMDB, nil)
	fallback := kinopoisk.New(cfg.Kinopoisk)
	failures := 0
	for _, e := range entries {
		if !*apply {
			fmt.Printf("would refresh movie %d\n", e.id)
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var d movie.Candidate
		switch {
		case e.tmdb != 0:
			d, err = primary.Details(ctx, e.tmdb)
		case cfg.Kinopoisk.Enabled():
			d, err = fallback.Details(ctx, e.kp)
		default:
			err = movie.ErrProviderUnavailable
		}
		if err != nil {
			failures++
			fmt.Fprintf(os.Stderr, "movie %d: catalogue details unavailable\n", e.id)
		} else {
			_, err = pool.Exec(ctx, `UPDATE movies SET runtime_min=coalesce(nullif($2,0),runtime_min),
   rating=coalesce(nullif($3::numeric,0),rating),
   rating_provider=CASE WHEN $3::numeric<>0 THEN $4 ELSE rating_provider END,
   rating_vote_count=CASE WHEN $3::numeric<>0 THEN $5 ELSE rating_vote_count END,
   updated_at=now() WHERE id=$1`, e.id, d.RuntimeMin, d.Rating, d.RatingProvider, d.RatingVoteCount)
			if err != nil {
				return fmt.Errorf("update movie %d: %w", e.id, err)
			}
			fmt.Printf("refreshed movie %d\n", e.id)
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if len(entries) > 0 {
		fmt.Printf("resume with --after-id=%d (retry failed IDs before advancing)\n", entries[len(entries)-1].id)
	}
	if failures > 0 {
		return fmt.Errorf("%d catalogue requests failed", failures)
	}
	return nil
}
