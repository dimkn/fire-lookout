-- Conditional-GET bookkeeping, so a poll that changes nothing costs a 304 with no body.
--
-- These two columns are OPAQUE HTTP VALIDATORS, echoed back to the provider verbatim in
-- If-None-Match / If-Modified-Since. They are deliberately NOT our RFC3339-UTC convention:
-- http_last_modified holds an HTTP-date ("Mon, 24 Aug 2026 10:00:00 GMT") exactly as the
-- server sent it. Do not normalise or parse them — the only correct value is the one the
-- provider gave us.

-- +goose Up
ALTER TABLE feed ADD COLUMN http_etag TEXT;
ALTER TABLE feed ADD COLUMN http_last_modified TEXT;

-- +goose Down
ALTER TABLE feed DROP COLUMN http_last_modified;
ALTER TABLE feed DROP COLUMN http_etag;
