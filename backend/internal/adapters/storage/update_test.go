package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

func TestUpdateFeedTouchesOnlyWhatWasGiven(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "2026-08-26T09:00:00Z", "2026-08-26T09:00:00Z", "")

	got, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if got.Enabled {
		t.Error("Enabled = true, want it paused")
	}
	// Everything else must survive untouched, including the poll bookkeeping.
	if got.Title != "GitHub" || got.URL != "https://a.test/feed" {
		t.Errorf("feed = %+v, want title and url unchanged", got)
	}
	if got.RefreshInterval != domain.DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want it unchanged", got.RefreshInterval)
	}
	if got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(mustTime(t, "2026-08-26T09:00:00Z")) {
		t.Errorf("LastSuccessAt = %v, want it unchanged", got.LastSuccessAt)
	}
}

func TestUpdateFeedTogglesBackOn(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")

	if _, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(false)}); err != nil {
		t.Fatalf("pausing: %v", err)
	}
	got, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(true)})
	if err != nil {
		t.Fatalf("resuming: %v", err)
	}

	if !got.Enabled {
		t.Error("Enabled = false, want it polling again")
	}
}

// A paused feed stops being due; switching it back on makes it due again immediately.
func TestUpdateFeedControlsWhetherAFeedIsDue(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	now := mustTime(t, "2026-08-26T12:00:00Z")

	if due, _ := repo.DueFeeds(context.Background(), now); len(due) != 1 {
		t.Fatalf("a fresh feed is not due, want it due")
	}

	if _, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(false)}); err != nil {
		t.Fatalf("pausing: %v", err)
	}
	if due, _ := repo.DueFeeds(context.Background(), now); len(due) != 0 {
		t.Errorf("a paused feed is still due, want polling to stop at once")
	}

	if _, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(true)}); err != nil {
		t.Fatalf("resuming: %v", err)
	}
	if due, _ := repo.DueFeeds(context.Background(), now); len(due) != 1 {
		t.Errorf("a resumed feed is not due, want polling to start again")
	}
}

func TestUpdateFeedChangesEveryFieldItIsGiven(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	interval := 30 * time.Second

	got, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{
		Title:           strPtr("GitHub Status"),
		RefreshInterval: &interval,
		Enabled:         boolPtr(false),
	})
	if err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if got.Title != "GitHub Status" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.RefreshInterval != 30*time.Second {
		t.Errorf("RefreshInterval = %v, want 30s", got.RefreshInterval)
	}
	if got.Enabled {
		t.Error("Enabled = true, want false")
	}
}

func TestUpdateFeedBumpsUpdatedAt(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	before := mustTime(t, "2026-08-02T00:00:00Z") // what insertFeed stores

	got, err := repo.UpdateFeed(context.Background(), 1, domain.UpdateFeedInput{Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if !got.UpdatedAt.After(before) {
		t.Errorf("UpdatedAt = %v, want it moved past %v", got.UpdatedAt, before)
	}
}

func TestUpdateFeedUnknownIsNotFound(t *testing.T) {
	repo, _ := newTestRepo(t)

	_, err := repo.UpdateFeed(context.Background(), 404, domain.UpdateFeedInput{Enabled: boolPtr(false)})

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestUpdateFeedRejectsATakenName(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed", "", "", "")
	insertFeed(t, db, 2, "Datadog", "https://b.test/feed", "", "", "")

	_, err := repo.UpdateFeed(context.Background(), 2, domain.UpdateFeedInput{Title: strPtr("github")})

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Errorf("error = %v, want ErrDuplicateTitle from the unique index", err)
	}
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
