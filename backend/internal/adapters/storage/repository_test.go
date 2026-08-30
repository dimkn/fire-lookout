package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return db
}

func newTestRepo(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewRepository(db), db
}

// insertFeed writes a feed row directly, bypassing the repository (which has no writes
// yet). Empty strings become SQL NULL for the nullable health columns.
func insertFeed(t *testing.T, db *sql.DB, id int64, title, url string, lastFetched, lastSuccess, lastErr string) {
	t.Helper()

	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO feed (id, url, title, group_id, enabled, refresh_interval_sec,
		                  last_fetched_at, last_success_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, 0, 1, 300, ?, ?, ?, '2026-08-01T00:00:00Z', '2026-08-02T00:00:00Z')`,
		id, url, title, null(lastFetched), null(lastSuccess), null(lastErr))
	if err != nil {
		t.Fatalf("insert feed %d: %v", id, err)
	}
}

func insertItem(t *testing.T, db *sql.DB, id, feedID int64, title, published, status string) {
	t.Helper()

	null := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO feed_item (id, feed_id, guid, title, link, published_at, updated_at,
		                       content_html, content_text, current_status, fetched_at)
		VALUES (?, ?, ?, ?, 'https://example.test/i', ?, NULL,
		        '<p>body</p>', 'body', ?, '2026-08-18T09:00:00Z')`,
		id, feedID, "guid-"+title, title, null(published), null(status))
	if err != nil {
		t.Fatalf("insert item %d: %v", id, err)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func TestMigrateSeedsUngrouped(t *testing.T) {
	db := newTestDB(t)

	var name string
	row := db.QueryRowContext(context.Background(), `SELECT name FROM feed_group WHERE id = ?`, domain.UngroupedID)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("query sentinel group: %v", err)
	}
	if name != "Ungrouped" {
		t.Errorf("sentinel group name = %q, want %q", name, "Ungrouped")
	}
}

// The FK pragma in the DSN is load-bearing: ON DELETE CASCADE is silently a no-op
// without it.
func TestForeignKeysAreEnforced(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://githubstatus.test/history.atom", "", "", "")
	insertItem(t, db, 10, 1, "Incident", "2026-08-18T08:00:00Z", "resolved")

	if _, err := db.ExecContext(context.Background(), `DELETE FROM feed WHERE id = 1`); err != nil {
		t.Fatalf("delete feed: %v", err)
	}

	items, err := repo.ListLatestItems(context.Background())
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items after deleting their feed, want 0 (cascade did not fire)", len(items))
	}
}

func TestListFeedsMapsEveryColumn(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://githubstatus.test/history.atom",
		"2026-08-18T09:05:00Z", "2026-08-18T09:00:00Z", "502 Bad Gateway")
	insertFeed(t, db, 2, "Never polled", "https://never.test/feed.xml", "", "", "")

	feeds, err := repo.ListFeeds(context.Background())
	if err != nil {
		t.Fatalf("ListFeeds() error = %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("got %d feeds, want 2", len(feeds))
	}

	polled := feeds[0]
	if polled.ID != 1 || polled.Title != "GitHub" || polled.URL != "https://githubstatus.test/history.atom" {
		t.Errorf("feed = %+v, want id/title/url from the row", polled)
	}
	if polled.GroupID != domain.UngroupedID {
		t.Errorf("GroupID = %d, want %d", polled.GroupID, domain.UngroupedID)
	}
	if !polled.Enabled {
		t.Error("Enabled = false, want true")
	}
	if polled.RefreshInterval != 300*time.Second {
		t.Errorf("RefreshInterval = %v, want 5m", polled.RefreshInterval)
	}
	if polled.LastFetchedAt == nil || !polled.LastFetchedAt.Equal(mustTime(t, "2026-08-18T09:05:00Z")) {
		t.Errorf("LastFetchedAt = %v, want 2026-08-18T09:05:00Z", polled.LastFetchedAt)
	}
	if polled.LastSuccessAt == nil || !polled.LastSuccessAt.Equal(mustTime(t, "2026-08-18T09:00:00Z")) {
		t.Errorf("LastSuccessAt = %v, want 2026-08-18T09:00:00Z", polled.LastSuccessAt)
	}
	if polled.LastError != "502 Bad Gateway" {
		t.Errorf("LastError = %q, want %q", polled.LastError, "502 Bad Gateway")
	}
	if !polled.CreatedAt.Equal(mustTime(t, "2026-08-01T00:00:00Z")) {
		t.Errorf("CreatedAt = %v, want 2026-08-01T00:00:00Z", polled.CreatedAt)
	}

	// NULL health columns must arrive as nil / empty, not as zero-value timestamps.
	fresh := feeds[1]
	if fresh.LastFetchedAt != nil || fresh.LastSuccessAt != nil {
		t.Errorf("never-polled feed has LastFetchedAt=%v LastSuccessAt=%v, want both nil",
			fresh.LastFetchedAt, fresh.LastSuccessAt)
	}
	if fresh.LastError != "" {
		t.Errorf("LastError = %q, want empty", fresh.LastError)
	}
}

func TestListFeedsWithNoRows(t *testing.T) {
	repo, _ := newTestRepo(t)

	feeds, err := repo.ListFeeds(context.Background())
	if err != nil {
		t.Fatalf("ListFeeds() error = %v", err)
	}
	if len(feeds) != 0 {
		t.Errorf("got %d feeds, want 0", len(feeds))
	}
}

func TestGetFeed(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 7, "Datadog", "https://ddstatus.test/feed", "", "", "")

	feed, err := repo.GetFeed(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	if feed.Title != "Datadog" {
		t.Errorf("Title = %q, want %q", feed.Title, "Datadog")
	}
}

func TestGetFeedUnknownIsNotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.GetFeed(context.Background(), 404)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("error = %v, want it to wrap domain.ErrNotFound", err)
	}
}

func TestListLatestItemsReturnsNewestPerFeed(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Datadog", "https://b.test/feed", "", "", "")
	insertFeed(t, db, 3, "Quiet", "https://c.test/feed", "", "", "")

	insertItem(t, db, 10, 1, "old", "2026-08-10T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "newest", "2026-08-17T00:00:00Z", "investigating")
	insertItem(t, db, 12, 1, "middle", "2026-08-12T00:00:00Z", "monitoring")
	insertItem(t, db, 20, 2, "only", "2026-08-01T00:00:00Z", "maintenance")
	// feed 3 has no items at all.

	items, err := repo.ListLatestItems(context.Background())
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want one per feed that has any (2)", len(items))
	}

	byFeed := map[int64]domain.StatusItem{}
	for _, item := range items {
		byFeed[item.FeedID] = item
	}
	if got := byFeed[1].Title; got != "newest" {
		t.Errorf("feed 1 latest = %q, want %q", got, "newest")
	}
	if got := byFeed[1].Status; got != domain.StatusInvestigating {
		t.Errorf("feed 1 status = %q, want investigating", got)
	}
	if got := byFeed[2].Title; got != "only" {
		t.Errorf("feed 2 latest = %q, want %q", got, "only")
	}
}

func TestListLatestItemsBreaksTimestampTiesByID(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "first", "2026-08-17T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "second", "2026-08-17T00:00:00Z", "identified")

	items, err := repo.ListLatestItems(context.Background())
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want exactly 1 despite the tie", len(items))
	}
	if items[0].ID != 11 {
		t.Errorf("tie resolved to item %d, want the higher id (11)", items[0].ID)
	}
}

func TestListItemsNewestFirstWithLimit(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "oldest", "2026-08-10T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "newest", "2026-08-17T00:00:00Z", "investigating")
	insertItem(t, db, 12, 1, "middle", "2026-08-12T00:00:00Z", "monitoring")

	all, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	titles := []string{}
	for _, item := range all {
		titles = append(titles, item.Title)
	}
	want := []string{"newest", "middle", "oldest"}
	for i := range want {
		if i >= len(titles) || titles[i] != want[i] {
			t.Fatalf("titles = %v, want %v", titles, want)
		}
	}

	limited, err := repo.ListItems(context.Background(), 1, nil, 2)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(limited) != 2 || limited[0].Title != "newest" {
		t.Errorf("limited = %+v, want the 2 newest", limited)
	}
}

func TestListItemsSinceFilter(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "old", "2026-08-10T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "recent", "2026-08-17T00:00:00Z", "monitoring")

	since := mustTime(t, "2026-08-12T00:00:00Z")
	items, err := repo.ListItems(context.Background(), 1, &since, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "recent" {
		t.Errorf("items = %+v, want only the one at or after %v", items, since)
	}
}

func TestListItemsOtherFeedsAreExcluded(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Datadog", "https://b.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "mine", "2026-08-10T00:00:00Z", "resolved")
	insertItem(t, db, 20, 2, "theirs", "2026-08-11T00:00:00Z", "resolved")

	items, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "mine" {
		t.Errorf("items = %+v, want only feed 1's item", items)
	}
}

func TestListItemsMapsNullableColumns(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	// No published_at, no status: both NULL in the row.
	insertItem(t, db, 10, 1, "bare", "", "")

	items, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}

	item := items[0]
	if !item.PublishedAt.IsZero() || !item.UpdatedAt.IsZero() {
		t.Errorf("PublishedAt=%v UpdatedAt=%v, want both zero", item.PublishedAt, item.UpdatedAt)
	}
	if item.Status != domain.StatusUnknown {
		t.Errorf("Status = %q, want %q for a NULL column", item.Status, domain.StatusUnknown)
	}
	if !item.FetchedAt.Equal(mustTime(t, "2026-08-18T09:00:00Z")) {
		t.Errorf("FetchedAt = %v, want the stored value", item.FetchedAt)
	}
	if item.ContentText != "body" || item.ContentHTML != "<p>body</p>" {
		t.Errorf("content = %q / %q, want the stored values", item.ContentText, item.ContentHTML)
	}
	if item.GUID == "" {
		t.Error("GUID is empty, want the stored value")
	}
}

func TestListItemsUnparsableStatusFallsBackToUnknown(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "weird", "2026-08-10T00:00:00Z", "postmortem-scheduled")

	items, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if items[0].Status != domain.StatusUnknown {
		t.Errorf("Status = %q, want %q", items[0].Status, domain.StatusUnknown)
	}
}

// Items whose feed shipped no published_at must still sort — the query falls back to
// updated_at and then to our own fetched_at.
func TestListItemsFallsBackToFetchedAtForOrdering(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "dated", "2026-08-10T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "undated", "", "resolved") // fetched_at 2026-08-18

	items, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 2 || items[0].Title != "undated" {
		t.Errorf("items = %+v, want the undated (fetched later) one first", items)
	}
}
