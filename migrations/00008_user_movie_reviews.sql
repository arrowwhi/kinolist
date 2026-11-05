-- +goose Up
ALTER TABLE user_movies ADD COLUMN review TEXT NOT NULL DEFAULT '';
ALTER TABLE user_movies ADD CONSTRAINT user_movies_review_length CHECK (char_length(review) <= 5000);
CREATE INDEX user_movies_user_watched_idx ON user_movies (user_id, watched_at DESC) WHERE status = 'watched';
-- +goose Down
DROP INDEX user_movies_user_watched_idx;
ALTER TABLE user_movies DROP COLUMN review;
