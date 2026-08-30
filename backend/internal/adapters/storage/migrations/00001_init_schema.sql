-- Initial schema for the local status-page reader.
--
-- Conventions (see AGENTS.md §4 "Data model"):
--   * All time columns are TEXT, RFC3339, UTC, second-precision. Normalize on write:
--     t.UTC().Format(time.RFC3339). Uniform width keeps lexicographic order == chronological.
--   * FK enforcement is REQUIRED for ON DELETE actions below. Open the DB with
--     file:./data/fire-lookout.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)

-- +goose Up

-- Arbitrary, user-defined buckets ("Dev tools", "Analytics", ...).
CREATE TABLE feed_group (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    position   INTEGER NOT NULL DEFAULT 0,            -- display order
    created_at TEXT    NOT NULL                       -- RFC3339 UTC
);

-- Sentinel group: id 0 means "ungrouped" and is the DEFAULT for feed.group_id.
-- It must exist so ON DELETE SET DEFAULT always has a valid FK target.
INSERT INTO feed_group (id, name, position, created_at)
VALUES (0, 'Ungrouped', -1, '2026-08-06T00:00:00Z');

-- Guard the sentinel: it may never be deleted.
-- +goose StatementBegin
CREATE TRIGGER feed_group_protect_ungrouped
BEFORE DELETE ON feed_group
WHEN OLD.id = 0
BEGIN
    SELECT RAISE(ABORT, 'cannot delete the Ungrouped group');
END;
-- +goose StatementEnd

-- One row per subscribed RSS/Atom endpoint.
CREATE TABLE feed (
    id                   INTEGER PRIMARY KEY,
    url                  TEXT    NOT NULL UNIQUE,        -- the RSS/Atom endpoint
    title                TEXT    NOT NULL,               -- user override or feed <title>
    group_id             INTEGER NOT NULL DEFAULT 0      -- 0 = Ungrouped
                             REFERENCES feed_group(id) ON DELETE SET DEFAULT,
    enabled              INTEGER NOT NULL DEFAULT 1,      -- 0/1: pause polling
    refresh_interval_sec INTEGER NOT NULL DEFAULT 300,    -- poll cadence
    last_fetched_at      TEXT,                            -- our clock: last poll ATTEMPT
    last_success_at      TEXT,                            -- our clock: last SUCCESS
    last_error           TEXT,                            -- last failure message, else NULL
    created_at           TEXT    NOT NULL,
    updated_at           TEXT    NOT NULL
);
CREATE INDEX idx_feed_group ON feed(group_id);

-- One row per incident / status entry (Atom <entry> / RSS <item>).
CREATE TABLE feed_item (
    id             INTEGER PRIMARY KEY,
    feed_id        INTEGER NOT NULL REFERENCES feed(id) ON DELETE CASCADE,
    guid           TEXT    NOT NULL,                 -- Atom <id> / RSS <guid> / fallback link
    title          TEXT    NOT NULL,
    link           TEXT,                             -- permalink to the incident
    published_at   TEXT,                             -- RFC3339 UTC
    updated_at     TEXT,                             -- RFC3339 UTC
    content_html   TEXT,                             -- raw body, all updates (source of truth)
    content_text   TEXT,                             -- stripped plaintext for preview/search
    current_status TEXT,                             -- derived enum, best-effort, nullable
    fetched_at     TEXT    NOT NULL,                 -- RFC3339 UTC: when last stored/updated
    UNIQUE (feed_id, guid)                           -- idempotent upserts across polls
);
CREATE INDEX idx_item_feed_time ON feed_item(feed_id, published_at DESC);

-- +goose Down
DROP TABLE IF EXISTS feed_item;
DROP TABLE IF EXISTS feed;
DROP TRIGGER IF EXISTS feed_group_protect_ungrouped;
DROP TABLE IF EXISTS feed_group;
