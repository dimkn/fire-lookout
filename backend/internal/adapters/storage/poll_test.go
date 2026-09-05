package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

// setPollState fills in the columns the poller owns, so the due-query can be exercised
// without going through RecordPoll.
func setPollState(t *testing.T, db *sql.DB, id int64, lastFetched string, intervalSec int, enabled bool) {
	t.Helper()

	on := 0
	if enabled {
		on = 1
	}
	var fetched any
	if lastFetched != "" {
		fetched = lastFetched
	}
	_, err := db.ExecContext(context.Background(),
		`UPDATE feed SET last_fetched_at = ?, refresh_interval_sec = ?, enabled = ? WHERE id = ?`,
		fetched, intervalSec, on, id)
	if err != nil {
		t.Fatalf("set poll state for feed %d: %v", id, err)
	}
}

func dueTitles(t *testing.T, repo *Repository, now string) []string {
	t.Helper()

	feeds, err := repo.DueFeeds(context.Background(), mustTime(t, now))
	if err != nil {
		t.Fatalf("DueFeeds() error = %v", err)
	}
	titles := make([]string, 0, len(feeds))
	for _, f := range feeds {
		titles = append(titles, f.Title)
	}
	return titles
}

func TestDueFeedsPicksWhatIsActuallyDue(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Never polled", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Due", "https://b.test/feed", "2026-08-24T11:50:00Z", "", "")
	insertFeed(t, db, 3, "Not yet", "https://c.test/feed", "2026-08-24T11:59:00Z", "", "")
	insertFeed(t, db, 4, "Paused", "https://d.test/feed", "2026-08-01T00:00:00Z", "", "")

	setPollState(t, db, 1, "", 300, true)
	setPollState(t, db, 2, "2026-08-24T11:50:00Z", 300, true) // 10 min ago, cadence 5 min
	setPollState(t, db, 3, "2026-08-24T11:59:00Z", 300, true) // 1 min ago, cadence 5 min
	setPollState(t, db, 4, "2026-08-01T00:00:00Z", 300, false)

	got := dueTitles(t, repo, "2026-08-24T12:00:00Z")

	if len(got) != 2 {
		t.Fatalf("due = %v, want exactly the never-polled and the overdue one", got)
	}
	if got[0] != "Never polled" && got[1] != "Never polled" {
		t.Errorf("due = %v, want the never-polled feed included", got)
	}
	for _, title := range got {
		if title == "Not yet" {
			t.Error("a feed inside its cadence was reported as due")
		}
		if title == "Paused" {
			t.Error("a disabled feed was reported as due — enabled must gate polling")
		}
	}
}

// The switch is the whole reason enabled exists: a paused feed is never fetched, however
// stale it gets.
func TestDueFeedsNeverReturnsADisabledFeed(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Paused", "https://a.test/feed", "", "", "")
	setPollState(t, db, 1, "", 300, false)

	if got := dueTitles(t, repo, "2030-01-01T00:00:00Z"); len(got) != 0 {
		t.Errorf("due = %v, want nothing", got)
	}
}

func TestDueFeedsHonoursEachFeedsOwnCadence(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Brisk", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Relaxed", "https://b.test/feed", "", "", "")
	setPollState(t, db, 1, "2026-08-24T11:59:00Z", 30, true)   // 60s ago, cadence 30s -> due
	setPollState(t, db, 2, "2026-08-24T11:59:00Z", 3600, true) // 60s ago, cadence 1h -> not due

	got := dueTitles(t, repo, "2026-08-24T12:00:00Z")

	if len(got) != 1 || got[0] != "Brisk" {
		t.Errorf("due = %v, want only the brisk feed", got)
	}
}

func TestDueFeedsCarriesTheStoredValidators(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	if _, err := db.ExecContext(context.Background(),
		`UPDATE feed SET http_etag = ?, http_last_modified = ? WHERE id = 1`,
		`W/"abc"`, "Mon, 24 Aug 2026 10:00:00 GMT"); err != nil {
		t.Fatalf("set validators: %v", err)
	}

	feeds, err := repo.DueFeeds(context.Background(), mustTime(t, "2026-08-24T12:00:00Z"))
	if err != nil {
		t.Fatalf("DueFeeds() error = %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("got %d feeds, want 1", len(feeds))
	}
	if feeds[0].ETag != `W/"abc"` {
		t.Errorf("ETag = %q, want it read back verbatim", feeds[0].ETag)
	}
	if feeds[0].LastModified != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Errorf("LastModified = %q, want the HTTP-date unchanged", feeds[0].LastModified)
	}
}

func pollItem(guid, title, text string, status domain.Status, published string) domain.StatusItem {
	item := domain.StatusItem{
		GUID: guid, Title: title, ContentText: text, ContentHTML: "<p>" + text + "</p>",
		Status: status, Link: "https://a.test/i/" + guid,
	}
	if published != "" {
		item.PublishedAt, _ = time.Parse(time.RFC3339, published)
	}
	item.FetchedAt = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	return item
}

func TestSaveItemsStoresEntries(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	items := []domain.StatusItem{
		pollItem("i-1", "Elevated errors", "Investigating - looking into it.", domain.StatusInvestigating, "2026-08-24T11:00:00Z"),
		pollItem("i-2", "Old news", "Resolved - all clear.", domain.StatusResolved, "2026-08-20T11:00:00Z"),
	}

	if err := repo.SaveItems(context.Background(), 1, items); err != nil {
		t.Fatalf("SaveItems() error = %v", err)
	}

	stored, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored %d items, want 2", len(stored))
	}
	if stored[0].GUID != "i-1" || stored[0].Status != domain.StatusInvestigating {
		t.Errorf("newest item = %+v, want i-1 investigating", stored[0])
	}
	if stored[0].ContentText != "Investigating - looking into it." {
		t.Errorf("ContentText = %q, want it stored", stored[0].ContentText)
	}
}

// Status pages append updates to the SAME entry, so a second poll must revise it rather
// than create a duplicate.
func TestSaveItemsRevisesAnEntryInPlace(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	first := pollItem("i-1", "Elevated errors", "Investigating - looking into it.", domain.StatusInvestigating, "2026-08-24T11:00:00Z")
	if err := repo.SaveItems(context.Background(), 1, []domain.StatusItem{first}); err != nil {
		t.Fatalf("first SaveItems() error = %v", err)
	}

	revised := pollItem("i-1", "Elevated errors", "Resolved - service is back.", domain.StatusResolved, "2026-08-24T11:00:00Z")
	if err := repo.SaveItems(context.Background(), 1, []domain.StatusItem{revised}); err != nil {
		t.Fatalf("second SaveItems() error = %v", err)
	}

	stored, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored %d items, want the one entry revised in place", len(stored))
	}
	if stored[0].Status != domain.StatusResolved {
		t.Errorf("Status = %q, want the revision to win", stored[0].Status)
	}
	if stored[0].ContentText != "Resolved - service is back." {
		t.Errorf("ContentText = %q, want the newer body", stored[0].ContentText)
	}
}

func TestSaveItemsWithNothingToSave(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	if err := repo.SaveItems(context.Background(), 1, nil); err != nil {
		t.Errorf("SaveItems() error = %v, want an empty list to be a no-op", err)
	}
}

func TestSaveItemsKeepsAbsentFieldsNull(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	bare := domain.StatusItem{
		GUID: "bare", Title: "No detail",
		Status:    domain.StatusUnknown,
		FetchedAt: mustTime(t, "2026-08-24T12:00:00Z"),
	}
	if err := repo.SaveItems(context.Background(), 1, []domain.StatusItem{bare}); err != nil {
		t.Fatalf("SaveItems() error = %v", err)
	}

	stored, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if !stored[0].PublishedAt.IsZero() || !stored[0].UpdatedAt.IsZero() {
		t.Errorf("timestamps = %v / %v, want both zero", stored[0].PublishedAt, stored[0].UpdatedAt)
	}
	if stored[0].Link != "" || stored[0].ContentText != "" {
		t.Errorf("optional fields = %q / %q, want empty", stored[0].Link, stored[0].ContentText)
	}
}

func feedState(t *testing.T, repo *Repository, id int64) domain.Feed {
	t.Helper()
	feed, err := repo.GetFeed(context.Background(), id)
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	return feed
}

func TestRecordPollUpdatedIsASuccess(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "2026-08-01T00:00:00Z", "2026-08-01T00:00:00Z", "an old failure")

	at := mustTime(t, "2026-08-24T12:00:00Z")
	err := repo.RecordPoll(context.Background(), domain.PollResult{
		FeedID: 1, At: at, Outcome: domain.PollUpdated,
		ETag: `W/"new"`, LastModified: "Mon, 24 Aug 2026 10:00:00 GMT",
	})
	if err != nil {
		t.Fatalf("RecordPoll() error = %v", err)
	}

	got := feedState(t, repo, 1)
	if got.LastFetchedAt == nil || !got.LastFetchedAt.Equal(at) {
		t.Errorf("LastFetchedAt = %v, want %v", got.LastFetchedAt, at)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(at) {
		t.Errorf("LastSuccessAt = %v, want %v", got.LastSuccessAt, at)
	}
	if got.LastError != "" {
		t.Errorf("LastError = %q, want a success to clear it", got.LastError)
	}
	if got.ETag != `W/"new"` || got.LastModified != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Errorf("validators = %q / %q, want the fresh ones", got.ETag, got.LastModified)
	}
}

func TestRecordPollNotModifiedIsAlsoASuccess(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "an old failure")

	at := mustTime(t, "2026-08-24T12:00:00Z")
	err := repo.RecordPoll(context.Background(), domain.PollResult{
		FeedID: 1, At: at, Outcome: domain.PollNotModified, ETag: `W/"same"`,
	})
	if err != nil {
		t.Fatalf("RecordPoll() error = %v", err)
	}

	got := feedState(t, repo, 1)
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(at) {
		t.Errorf("LastSuccessAt = %v, want a 304 to count as success", got.LastSuccessAt)
	}
	if got.LastError != "" {
		t.Errorf("LastError = %q, want it cleared", got.LastError)
	}
}

// A 429 says nothing about the feed: only the attempt is recorded, so it waits a full
// interval and the card shows no error.
func TestRecordPollRateLimitedTouchesOnlyTheAttempt(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed",
		"2026-08-24T11:00:00Z", "2026-08-24T11:00:00Z", "")

	at := mustTime(t, "2026-08-24T12:00:00Z")
	if err := repo.RecordPoll(context.Background(), domain.PollResult{
		FeedID: 1, At: at, Outcome: domain.PollRateLimited,
	}); err != nil {
		t.Fatalf("RecordPoll() error = %v", err)
	}

	got := feedState(t, repo, 1)
	if got.LastFetchedAt == nil || !got.LastFetchedAt.Equal(at) {
		t.Errorf("LastFetchedAt = %v, want the attempt recorded so it waits its cadence", got.LastFetchedAt)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(mustTime(t, "2026-08-24T11:00:00Z")) {
		t.Errorf("LastSuccessAt = %v, want it untouched", got.LastSuccessAt)
	}
	if got.LastError != "" {
		t.Errorf("LastError = %q, want a 429 recorded as no error at all", got.LastError)
	}
}

func TestRecordPollFailedKeepsTheLastSuccess(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed",
		"2026-08-24T11:00:00Z", "2026-08-24T11:00:00Z", "")

	at := mustTime(t, "2026-08-24T12:00:00Z")
	if err := repo.RecordPoll(context.Background(), domain.PollResult{
		FeedID: 1, At: at, Outcome: domain.PollFailed, Error: "dial tcp: i/o timeout",
	}); err != nil {
		t.Fatalf("RecordPoll() error = %v", err)
	}

	got := feedState(t, repo, 1)
	if got.LastError != "dial tcp: i/o timeout" {
		t.Errorf("LastError = %q, want the reason stored", got.LastError)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(mustTime(t, "2026-08-24T11:00:00Z")) {
		t.Errorf("LastSuccessAt = %v, want the last good poll preserved", got.LastSuccessAt)
	}
}

func TestRecordPollRejectsAnUnknownOutcome(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	err := repo.RecordPoll(context.Background(), domain.PollResult{
		FeedID: 1, At: mustTime(t, "2026-08-24T12:00:00Z"), Outcome: "nonsense",
	})

	if err == nil {
		t.Error("RecordPoll() accepted an unknown outcome, want an error")
	}
}

func TestPruneItemsDropsOnlyOldEntries(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "ancient", "2026-07-01T00:00:00Z", "resolved")
	insertItem(t, db, 11, 1, "recent", "2026-08-23T00:00:00Z", "resolved")

	removed, err := repo.PruneItems(context.Background(), 1, mustTime(t, "2026-08-17T00:00:00Z"))
	if err != nil {
		t.Fatalf("PruneItems() error = %v", err)
	}
	if removed != 1 {
		t.Errorf("removed %d items, want 1", removed)
	}

	left, err := repo.ListItems(context.Background(), 1, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(left) != 1 || left[0].Title != "recent" {
		t.Errorf("remaining = %+v, want only the recent entry", left)
	}
}

func TestPruneItemsLeavesOtherFeedsAlone(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Datadog", "https://b.test/feed", "", "", "")
	insertItem(t, db, 10, 1, "mine-old", "2026-07-01T00:00:00Z", "resolved")
	insertItem(t, db, 20, 2, "theirs-old", "2026-07-01T00:00:00Z", "resolved")

	if _, err := repo.PruneItems(context.Background(), 1, mustTime(t, "2026-08-17T00:00:00Z")); err != nil {
		t.Fatalf("PruneItems() error = %v", err)
	}

	theirs, err := repo.ListItems(context.Background(), 2, nil, 50)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(theirs) != 1 {
		t.Errorf("other feed has %d items, want its own untouched", len(theirs))
	}
}
