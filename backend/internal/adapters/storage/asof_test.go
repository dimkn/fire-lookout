package storage

import (
	"context"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

// farFuture is the as-of bound for tests that are not about future-dated entries: it keeps
// every stored entry in scope, so those tests keep asserting what they always asserted.
var farFuture = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

// insertDatedItem writes one entry with an explicit published_at and status.
func insertDatedItem(t *testing.T, repo *Repository, feedID int64, guid, title, published string, status domain.Status) {
	t.Helper()

	item := domain.StatusItem{
		GUID: guid, Title: title, Status: status,
		PublishedAt: mustTime(t, published),
		FetchedAt:   mustTime(t, "2026-08-26T12:00:00Z"),
	}
	if err := repo.SaveItems(context.Background(), feedID, []domain.StatusItem{item}); err != nil {
		t.Fatalf("SaveItems(%s): %v", guid, err)
	}
}

// The Cloudflare case. Statuspage dates a scheduled-maintenance entry when the window will
// OPEN, so a feed routinely carries entries days ahead. Letting one of those be "latest"
// made a healthy system show as degraded, and hid the real incident underneath it.
func TestListLatestItemsIgnoresEntriesStillToCome(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Cloudflare", "https://cf.test/feed", "", "", "")

	insertDatedItem(t, repo, 1, "future-window", "EWR (Newark) on 2026-09-02",
		"2026-09-02T05:00:00Z", domain.StatusMaintenance)
	insertDatedItem(t, repo, 1, "real-incident", "Increased Latency",
		"2026-08-25T14:52:41Z", domain.StatusResolved)

	items, err := repo.ListLatestItems(context.Background(), mustTime(t, "2026-08-26T12:00:00Z"))
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want exactly one per feed", len(items))
	}
	if items[0].GUID != "real-incident" {
		t.Errorf("latest = %q, want the newest entry that has actually happened", items[0].GUID)
	}
	if items[0].Status != domain.StatusResolved {
		t.Errorf("status = %q, want resolved — the light must not come from next week", items[0].Status)
	}
}

// Once the window opens, the same entry counts: an in-progress maintenance still turns the
// card yellow.
func TestListLatestItemsCountsAWindowOnceItHasOpened(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Cloudflare", "https://cf.test/feed", "", "", "")

	insertDatedItem(t, repo, 1, "window", "GRU (São Paulo)",
		"2026-08-26T04:00:00Z", domain.StatusMaintenance)
	insertDatedItem(t, repo, 1, "older", "Increased Latency",
		"2026-08-25T14:52:41Z", domain.StatusResolved)

	items, err := repo.ListLatestItems(context.Background(), mustTime(t, "2026-08-26T12:00:00Z"))
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 1 || items[0].GUID != "window" {
		t.Fatalf("latest = %+v, want the started maintenance window", items)
	}
	if items[0].Status != domain.StatusMaintenance {
		t.Errorf("status = %q, want maintenance while the window is open", items[0].Status)
	}
}

// A feed whose every entry is still to come has said nothing about now, so it yields no
// latest item at all — the overview then falls through to its polled-successfully rule.
func TestListLatestItemsSkipsAFeedWithOnlyFutureEntries(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "All announcements", "https://a.test/feed", "", "", "")

	insertDatedItem(t, repo, 1, "w1", "Window one", "2026-09-01T00:00:00Z", domain.StatusMaintenance)
	insertDatedItem(t, repo, 1, "w2", "Window two", "2026-09-02T00:00:00Z", domain.StatusMaintenance)

	items, err := repo.ListLatestItems(context.Background(), mustTime(t, "2026-08-26T12:00:00Z"))
	if err != nil {
		t.Fatalf("ListLatestItems() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %+v, want nothing: the feed has said nothing about now", items)
	}
}

func TestListItemsHidesEntriesStillToCome(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Cloudflare", "https://cf.test/feed", "", "", "")

	insertDatedItem(t, repo, 1, "future", "Next week's window", "2026-09-02T05:00:00Z", domain.StatusMaintenance)
	insertDatedItem(t, repo, 1, "today", "Increased Latency", "2026-08-25T14:52:41Z", domain.StatusResolved)
	insertDatedItem(t, repo, 1, "older", "Earlier blip", "2026-08-20T10:00:00Z", domain.StatusResolved)

	got, err := repo.ListItems(context.Background(), domain.ItemQuery{
		FeedID: 1, AsOf: mustTime(t, "2026-08-26T12:00:00Z"), Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}

	guids := make([]string, 0, len(got))
	for _, item := range got {
		guids = append(guids, item.GUID)
	}
	if len(guids) != 2 || guids[0] != "today" || guids[1] != "older" {
		t.Errorf("items = %v, want the two that have happened, newest first", guids)
	}
}

// The two bounds work together rather than cancelling out.
func TestListItemsCombinesSinceAndAsOf(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "Cloudflare", "https://cf.test/feed", "", "", "")

	insertDatedItem(t, repo, 1, "ancient", "Ancient", "2026-08-01T00:00:00Z", domain.StatusResolved)
	insertDatedItem(t, repo, 1, "recent", "Recent", "2026-08-25T00:00:00Z", domain.StatusResolved)
	insertDatedItem(t, repo, 1, "future", "Future", "2026-09-02T00:00:00Z", domain.StatusMaintenance)

	since := mustTime(t, "2026-08-20T00:00:00Z")
	got, err := repo.ListItems(context.Background(), domain.ItemQuery{
		FeedID: 1, Since: &since, AsOf: mustTime(t, "2026-08-26T12:00:00Z"), Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(got) != 1 || got[0].GUID != "recent" {
		t.Errorf("items = %+v, want only the one inside both bounds", got)
	}
}
