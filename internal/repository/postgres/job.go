package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arrowwhi/kinolist/internal/domain/enrichment"
	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/pkg/transactions"
)

type JobRepository struct {
	tx *transactions.Manager
}

func NewJobRepository(tx *transactions.Manager) *JobRepository {
	return &JobRepository{tx: tx}
}

const jobColumns = `
	id, user_id, kind, status, source_url, raw_title, query_title,
	coalesce(query_year, 0), candidates, attempts, last_error,
	coalesce(chat_id, 0), run_after, created_at, updated_at`

func (r *JobRepository) Create(ctx context.Context, j enrichment.Job) (enrichment.Job, error) {
	candidates, err := marshalCandidates(j.Candidates)
	if err != nil {
		return enrichment.Job{}, err
	}

	const q = `
		INSERT INTO enrichment_jobs (user_id, kind, status, source_url, raw_title,
		                             query_title, query_year, candidates, chat_id, run_after)
		VALUES ($1, $2, $3, $4, $5, $6, nullif($7, 0)::smallint, $8, nullif($9, 0), $10)
		RETURNING ` + jobColumns

	row := r.tx.Querier(ctx).QueryRow(ctx, q,
		j.UserID, j.Kind, j.Status, j.SourceURL, j.RawTitle,
		j.QueryTitle, j.QueryYear, candidates, j.ChatID, j.RunAfter,
	)

	out, err := scanJob(row)
	if err != nil {
		return enrichment.Job{}, fmt.Errorf("create enrichment job: %w", err)
	}

	return out, nil
}

func (r *JobRepository) Get(ctx context.Context, id int64) (enrichment.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM enrichment_jobs WHERE id = $1`

	out, err := scanJob(r.tx.Querier(ctx).QueryRow(ctx, q, id))
	if err != nil {
		if isNoRows(err) {
			return enrichment.Job{}, enrichment.ErrJobNotFound
		}
		return enrichment.Job{}, fmt.Errorf("get enrichment job: %w", err)
	}

	return out, nil
}

// GetForUser не даёт подтвердить чужую задачу по угаданному id.
func (r *JobRepository) GetForUser(ctx context.Context, userID, id int64) (enrichment.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM enrichment_jobs WHERE id = $1 AND user_id = $2`

	out, err := scanJob(r.tx.Querier(ctx).QueryRow(ctx, q, id, userID))
	if err != nil {
		if isNoRows(err) {
			return enrichment.Job{}, enrichment.ErrJobNotFound
		}
		return enrichment.Job{}, fmt.Errorf("get enrichment job for user: %w", err)
	}

	return out, nil
}

func (r *JobRepository) Update(ctx context.Context, j enrichment.Job) error {
	candidates, err := marshalCandidates(j.Candidates)
	if err != nil {
		return err
	}

	const q = `
		UPDATE enrichment_jobs
		SET status      = $2,
		    source_url  = $3,
		    raw_title   = $4,
		    query_title = $5,
		    query_year  = nullif($6, 0)::smallint,
		    candidates  = $7,
		    attempts    = $8,
		    last_error  = $9,
		    run_after   = $10,
		    updated_at  = now()
		WHERE id = $1`

	tag, err := r.tx.Querier(ctx).Exec(ctx, q,
		j.ID, j.Status, j.SourceURL, j.RawTitle, j.QueryTitle, j.QueryYear,
		candidates, j.Attempts, truncateError(j.LastError), j.RunAfter,
	)
	if err != nil {
		return fmt.Errorf("update enrichment job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return enrichment.ErrJobNotFound
	}

	return nil
}

// Claim атомарно забирает готовые задачи. SKIP LOCKED позволяет нескольким
// воркерам разбирать очередь параллельно, не блокируя друг друга и не
// выдавая одну задачу дважды.
func (r *JobRepository) Claim(ctx context.Context, limit int) ([]enrichment.Job, error) {
	if limit <= 0 {
		return nil, nil
	}

	const q = `
		WITH claimed AS (
		    SELECT id
		    FROM enrichment_jobs
		    WHERE status = 'pending' AND run_after <= now()
		    ORDER BY run_after
		    LIMIT $1
		    FOR UPDATE SKIP LOCKED
		)
		UPDATE enrichment_jobs j
		SET status = 'processing', updated_at = now()
		FROM claimed
		WHERE j.id = claimed.id
		RETURNING j.id, j.user_id, j.kind, j.status, j.source_url, j.raw_title, j.query_title,
		          coalesce(j.query_year, 0), j.candidates, j.attempts, j.last_error,
		          coalesce(j.chat_id, 0), j.run_after, j.created_at, j.updated_at`

	rows, err := r.tx.Querier(ctx).Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("claim enrichment jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]enrichment.Job, 0, limit)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan claimed job: %w", err)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed jobs: %w", err)
	}

	return jobs, nil
}

func (r *JobRepository) GetSession(ctx context.Context, userID int64) (enrichment.Session, error) {
	const q = `
		SELECT user_id, state, coalesce(job_id, 0), chat_id, coalesce(message_id, 0), updated_at
		FROM bot_sessions WHERE user_id = $1`

	var s enrichment.Session
	err := r.tx.Querier(ctx).QueryRow(ctx, q, userID).
		Scan(&s.UserID, &s.State, &s.JobID, &s.ChatID, &s.MessageID, &s.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return enrichment.Session{UserID: userID, State: enrichment.SessionIdle}, nil
		}
		return enrichment.Session{}, fmt.Errorf("get bot session: %w", err)
	}

	return s, nil
}

func (r *JobRepository) SaveSession(ctx context.Context, s enrichment.Session) error {
	const q = `
		INSERT INTO bot_sessions (user_id, state, job_id, chat_id, message_id, updated_at)
		VALUES ($1, $2, nullif($3, 0), $4, nullif($5, 0), now())
		ON CONFLICT (user_id) DO UPDATE
		SET state = EXCLUDED.state, job_id = EXCLUDED.job_id,
		    chat_id = EXCLUDED.chat_id, message_id = EXCLUDED.message_id,
		    updated_at = now()`

	if _, err := r.tx.Querier(ctx).Exec(ctx, q, s.UserID, s.State, s.JobID, s.ChatID, s.MessageID); err != nil {
		return fmt.Errorf("save bot session: %w", err)
	}

	return nil
}

func (r *JobRepository) ClearSession(ctx context.Context, userID int64) error {
	if _, err := r.tx.Querier(ctx).Exec(ctx, `DELETE FROM bot_sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear bot session: %w", err)
	}
	return nil
}

func scanJob(row scanner) (enrichment.Job, error) {
	var (
		j   enrichment.Job
		raw []byte
	)

	err := row.Scan(
		&j.ID, &j.UserID, &j.Kind, &j.Status, &j.SourceURL, &j.RawTitle, &j.QueryTitle,
		&j.QueryYear, &raw, &j.Attempts, &j.LastError, &j.ChatID,
		&j.RunAfter, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return enrichment.Job{}, err
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &j.Candidates); err != nil {
			return enrichment.Job{}, fmt.Errorf("decode job candidates: %w", err)
		}
	}

	return j, nil
}

func marshalCandidates(candidates []movie.Candidate) ([]byte, error) {
	if len(candidates) == 0 {
		return []byte("[]"), nil
	}
	raw, err := json.Marshal(candidates)
	if err != nil {
		return nil, fmt.Errorf("encode job candidates: %w", err)
	}
	return raw, nil
}

// maxErrorLen ограничивает last_error: ответы внешних API бывают на мегабайт,
// а в колонке нужен только повод для разбора.
const maxErrorLen = 1000

func truncateError(s string) string {
	if len(s) <= maxErrorLen {
		return s
	}
	return s[:maxErrorLen]
}
