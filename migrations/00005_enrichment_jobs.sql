-- +goose Up
CREATE TABLE enrichment_jobs (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind         TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending',
    source_url   TEXT        NOT NULL DEFAULT '',
    raw_title    TEXT        NOT NULL DEFAULT '',
    query_title  TEXT        NOT NULL DEFAULT '',
    query_year   SMALLINT,
    candidates   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    attempts     SMALLINT    NOT NULL DEFAULT 0,
    last_error   TEXT        NOT NULL DEFAULT '',
    chat_id      BIGINT,
    run_after    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT enrichment_jobs_kind_valid   CHECK (kind IN ('reel', 'title')),
    CONSTRAINT enrichment_jobs_status_valid CHECK (status IN ('pending', 'processing', 'confirming', 'done', 'failed', 'cancelled'))
);

-- Воркер выбирает задачи именно по этой паре, частичный индекс держит его узким.
CREATE INDEX enrichment_jobs_claim_idx ON enrichment_jobs (run_after)
    WHERE status = 'pending';

CREATE INDEX enrichment_jobs_user_idx ON enrichment_jobs (user_id, created_at DESC);

-- +goose Down
DROP TABLE enrichment_jobs;
