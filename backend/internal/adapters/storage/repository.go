package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"fire-lookout/backend/internal/domain"
)

// Repository implements domain.StatusRepository on SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps an open database handle.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ListFeeds returns every subscribed feed, ordered by id so the result is stable.
// Display order is the application's business, not the adapter's.
func (r *Repository) ListFeeds(ctx context.Context) ([]domain.Feed, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, title, group_id, enabled, refresh_interval_sec,
		       last_fetched_at, last_success_at, last_error, created_at, updated_at
		FROM feed
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query feeds: %w", err)
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
		return nil, fmt.Errorf("iterate feeds: %w", err)
	}
	return feeds, nil
}

// GetFeed returns one feed, or an error wrapping domain.ErrNotFound.
func (r *Repository) GetFeed(ctx context.Context, id int64) (domain.Feed, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, url, title, group_id, enabled, refresh_interval_sec,
		       last_fetched_at, last_success_at, last_error, created_at, updated_at
		FROM feed
		WHERE id = ?`, id)

	feed, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Feed{}, fmt.Errorf("feed %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Feed{}, err
	}
	return feed, nil
}

// ListLatestItems returns the newest item of every feed that has one, in a single query.
// ROW_NUMBER() rather than a MAX() join, so feeds whose newest items share a timestamp
// still yield exactly one row (the higher id wins).
func (r *Repository) ListLatestItems(ctx context.Context) ([]domain.StatusItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, feed_id, guid, title, link, published_at, updated_at,
		       content_html, content_text, current_status, fetched_at
		FROM (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY feed_id
				ORDER BY COALESCE(published_at, updated_at, fetched_at) DESC, id DESC
			) AS rn
			FROM feed_item
		)
		WHERE rn = 1`)
	if err != nil {
		return nil, fmt.Errorf("query latest items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return collectItems(rows)
}

// ListItems returns one feed's items, newest first, capped at limit. A non-nil since
// restricts the result to items at or after that instant. Timestamps are compared as
// stored text, which is safe because every one is fixed-width RFC3339 UTC.
func (r *Repository) ListItems(ctx context.Context, feedID int64, since *time.Time, limit int) ([]domain.StatusItem, error) {
	var sinceArg any
	if since != nil {
		sinceArg = formatTime(*since)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, feed_id, guid, title, link, published_at, updated_at,
		       content_html, content_text, current_status, fetched_at
		FROM feed_item
		WHERE feed_id = ?
		  AND (? IS NULL OR COALESCE(published_at, updated_at, fetched_at) >= ?)
		ORDER BY COALESCE(published_at, updated_at, fetched_at) DESC, id DESC
		LIMIT ?`, feedID, sinceArg, sinceArg, limit)
	if err != nil {
		return nil, fmt.Errorf("query items for feed %d: %w", feedID, err)
	}
	defer func() { _ = rows.Close() }()

	return collectItems(rows)
}

// scanner is what sql.Row and sql.Rows have in common.
type scanner interface {
	Scan(dest ...any) error
}

func scanFeed(s scanner) (domain.Feed, error) {
	var (
		feed        domain.Feed
		enabled     int64
		intervalSec int64
		lastFetched sql.NullString
		lastSuccess sql.NullString
		lastError   sql.NullString
		createdAt   string
		updatedAt   string
	)

	if err := s.Scan(
		&feed.ID, &feed.URL, &feed.Title, &feed.GroupID, &enabled, &intervalSec,
		&lastFetched, &lastSuccess, &lastError, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Feed{}, err
		}
		return domain.Feed{}, fmt.Errorf("scan feed: %w", err)
	}

	feed.Enabled = enabled != 0
	feed.RefreshInterval = time.Duration(intervalSec) * time.Second
	feed.LastError = lastError.String

	var err error
	if feed.LastFetchedAt, err = parseNullTime(lastFetched); err != nil {
		return domain.Feed{}, fmt.Errorf("feed %d last_fetched_at: %w", feed.ID, err)
	}
	if feed.LastSuccessAt, err = parseNullTime(lastSuccess); err != nil {
		return domain.Feed{}, fmt.Errorf("feed %d last_success_at: %w", feed.ID, err)
	}
	if feed.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.Feed{}, fmt.Errorf("feed %d created_at: %w", feed.ID, err)
	}
	if feed.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.Feed{}, fmt.Errorf("feed %d updated_at: %w", feed.ID, err)
	}
	return feed, nil
}

func collectItems(rows *sql.Rows) ([]domain.StatusItem, error) {
	items := []domain.StatusItem{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	return items, nil
}

func scanItem(s scanner) (domain.StatusItem, error) {
	var (
		item        domain.StatusItem
		link        sql.NullString
		publishedAt sql.NullString
		updatedAt   sql.NullString
		contentHTML sql.NullString
		contentText sql.NullString
		status      sql.NullString
		fetchedAt   string
	)

	if err := s.Scan(
		&item.ID, &item.FeedID, &item.GUID, &item.Title, &link, &publishedAt, &updatedAt,
		&contentHTML, &contentText, &status, &fetchedAt,
	); err != nil {
		return domain.StatusItem{}, fmt.Errorf("scan status item: %w", err)
	}

	item.Link = link.String
	item.ContentHTML = contentHTML.String
	item.ContentText = contentText.String

	// current_status is a best-effort derivation stored by the poller: anything missing
	// or unrecognized reads back as unknown rather than failing the whole listing.
	item.Status = domain.Status(status.String)
	if !item.Status.Valid() {
		item.Status = domain.StatusUnknown
	}

	published, err := parseNullTime(publishedAt)
	if err != nil {
		return domain.StatusItem{}, fmt.Errorf("item %d published_at: %w", item.ID, err)
	}
	if published != nil {
		item.PublishedAt = *published
	}

	updated, err := parseNullTime(updatedAt)
	if err != nil {
		return domain.StatusItem{}, fmt.Errorf("item %d updated_at: %w", item.ID, err)
	}
	if updated != nil {
		item.UpdatedAt = *updated
	}

	if item.FetchedAt, err = parseTime(fetchedAt); err != nil {
		return domain.StatusItem{}, fmt.Errorf("item %d fetched_at: %w", item.ID, err)
	}
	return item, nil
}

// formatTime renders a timestamp the way every time column stores it: RFC3339, UTC,
// second precision. Uniform width keeps lexicographic order chronological.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}

func parseNullTime(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	t, err := parseTime(ns.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
