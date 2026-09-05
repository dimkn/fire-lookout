package storage

import (
	"context"
	"fmt"
	"time"

	"fire-lookout/backend/internal/domain"
)

// DueFeeds returns the enabled feeds whose cadence has elapsed, plus any never polled.
//
// Both sides of the comparison go through datetime(): SQLite renders it as
// "2026-08-24 12:00:00" while our columns hold "2026-08-24T12:00:00Z", so comparing a raw
// string against a converted one would silently never match.
func (r *Repository) DueFeeds(ctx context.Context, now time.Time) ([]domain.Feed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, title, group_id, enabled, refresh_interval_sec,
		       last_fetched_at, last_success_at, last_error, http_etag, http_last_modified,
		       created_at, updated_at
		FROM feed
		WHERE enabled = 1
		  AND (
		        last_fetched_at IS NULL
		        OR datetime(last_fetched_at, '+' || refresh_interval_sec || ' seconds') <= datetime(?)
		      )
		ORDER BY COALESCE(last_fetched_at, ''), id`, formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("query due feeds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	feeds := []domain.Feed{}
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, feed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due feeds: %w", err)
	}
	return feeds, nil
}

// SaveItems upserts a poll's entries in one transaction. The conflict target is the
// (feed_id, guid) unique index, so an incident that gained an update is revised in place
// instead of arriving as a duplicate.
func (r *Repository) SaveItems(ctx context.Context, feedID int64, items []domain.StatusItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin item upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO feed_item (feed_id, guid, title, link, published_at, updated_at,
		                       content_html, content_text, current_status, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(feed_id, guid) DO UPDATE SET
		    title          = excluded.title,
		    link           = excluded.link,
		    published_at   = excluded.published_at,
		    updated_at     = excluded.updated_at,
		    content_html   = excluded.content_html,
		    content_text   = excluded.content_text,
		    current_status = excluded.current_status,
		    fetched_at     = excluded.fetched_at`)
	if err != nil {
		return fmt.Errorf("prepare item upsert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, item := range items {
		_, err := stmt.ExecContext(ctx,
			feedID, item.GUID, item.Title, nullString(item.Link),
			nullTime(item.PublishedAt), nullTime(item.UpdatedAt),
			nullString(item.ContentHTML), nullString(item.ContentText),
			nullString(string(item.Status)), formatTime(item.FetchedAt),
		)
		if err != nil {
			return fmt.Errorf("upsert item %q: %w", item.GUID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit item upsert: %w", err)
	}
	return nil
}

// RecordPoll writes the bookkeeping for one attempt. Each outcome owns a different set of
// columns:
//
//   - updated / not_modified — a successful poll: attempt time, success time, fresh
//     validators, and last_error cleared;
//   - rate_limited — the attempt time only. The provider asked us to slow down, which says
//     nothing about the feed, so last_error and last_success_at are left exactly as they were;
//   - failed — attempt time and the reason, leaving last_success_at as the last time it did
//     work so the card can say "last worked at ...".
func (r *Repository) RecordPoll(ctx context.Context, result domain.PollResult) error {
	at := formatTime(result.At)

	var (
		query string
		args  []any
	)
	switch result.Outcome {
	case domain.PollUpdated, domain.PollNotModified:
		query = `
			UPDATE feed
			SET last_fetched_at = ?, last_success_at = ?, last_error = NULL,
			    http_etag = ?, http_last_modified = ?, updated_at = ?
			WHERE id = ?`
		args = []any{at, at, nullString(result.ETag), nullString(result.LastModified), at, result.FeedID}

	case domain.PollRateLimited:
		query = `UPDATE feed SET last_fetched_at = ?, updated_at = ? WHERE id = ?`
		args = []any{at, at, result.FeedID}

	case domain.PollFailed:
		query = `UPDATE feed SET last_fetched_at = ?, last_error = ?, updated_at = ? WHERE id = ?`
		args = []any{at, nullString(result.Error), at, result.FeedID}

	default:
		return fmt.Errorf("record poll for feed %d: unknown outcome %q", result.FeedID, result.Outcome)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("record poll for feed %d: %w", result.FeedID, err)
	}
	return nil
}

// PruneItems drops a feed's entries older than before, using the same timestamp fallback the
// listings order by.
func (r *Repository) PruneItems(ctx context.Context, feedID int64, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM feed_item
		WHERE feed_id = ?
		  AND COALESCE(published_at, updated_at, fetched_at) < ?`, feedID, formatTime(before))
	if err != nil {
		return 0, fmt.Errorf("prune items for feed %d: %w", feedID, err)
	}

	removed, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count pruned items for feed %d: %w", feedID, err)
	}
	return removed, nil
}

// nullString stores "" as SQL NULL, keeping "absent" and "empty" the same thing in columns
// the reader already treats as optional.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullTime stores a zero time as SQL NULL: the provider simply did not say.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}
