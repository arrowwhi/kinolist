-- +goose Up
ALTER TABLE movies ADD COLUMN rating_provider TEXT;
ALTER TABLE movies ADD COLUMN rating_vote_count INTEGER;
ALTER TABLE movies ADD CONSTRAINT movies_rating_provider_valid CHECK (rating_provider IN ('tmdb', 'kinopoisk'));
ALTER TABLE movies ADD CONSTRAINT movies_rating_votes_valid CHECK (rating_vote_count >= 0);
ALTER TABLE movies ADD CONSTRAINT movies_rating_votes_provider CHECK (rating_vote_count IS NULL OR rating_provider IS NOT NULL);
-- Existing ratings with both external IDs have ambiguous provenance; leave them unknown.
UPDATE movies SET rating_provider = CASE WHEN tmdb_id IS NOT NULL THEN 'tmdb' ELSE 'kinopoisk' END
WHERE rating IS NOT NULL AND (tmdb_id IS NULL OR kinopoisk_id IS NULL);
-- +goose Down
ALTER TABLE movies DROP CONSTRAINT movies_rating_votes_provider;
ALTER TABLE movies DROP COLUMN rating_vote_count;
ALTER TABLE movies DROP COLUMN rating_provider;
