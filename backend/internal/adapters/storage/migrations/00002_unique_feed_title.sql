-- Feed titles are the user-facing name of a system, so two feeds may not share one.
-- Case-insensitive: "GitHub" and "github" read as the same name to a human, and the
-- subscribe flow rejects the second with 409 rather than creating a confusing duplicate.
--
-- feed.url already carries its own UNIQUE constraint (see 00001).

-- +goose Up
CREATE UNIQUE INDEX idx_feed_title_unique ON feed(title COLLATE NOCASE);

-- +goose Down
DROP INDEX IF EXISTS idx_feed_title_unique;
