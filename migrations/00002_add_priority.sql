-- +goose Up
-- Fixed version: a DEFAULT lets Postgres backfill existing rows, so this
-- succeeds on a populated table. This is the version that ships and is
-- promoted to the source instance after validation on a branch.
ALTER TABLE notes ADD COLUMN priority INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE notes DROP COLUMN priority;
