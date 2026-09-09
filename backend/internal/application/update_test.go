package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

func updateService(repo *fakeRepo) *SubscriptionService {
	return NewSubscriptionService(repo, &fakeFetcher{})
}

// The toggle is a one-field update: everything else must be left alone.
func TestUpdateFeedTogglesEnabledOnly(t *testing.T) {
	repo := &fakeRepo{
		feeds:       []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}},
		updatedFeed: domain.Feed{Title: "GitHub", Enabled: false},
	}

	got, err := updateService(repo).UpdateFeed(context.Background(), 1,
		domain.UpdateFeedInput{Enabled: ptr(false)})
	if err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if got.Enabled {
		t.Error("returned feed is still enabled, want it paused")
	}
	if repo.updated.Enabled == nil || *repo.updated.Enabled {
		t.Errorf("repository received %+v, want enabled=false", repo.updated)
	}
	if repo.updated.Title != nil || repo.updated.GroupID != nil || repo.updated.RefreshInterval != nil {
		t.Errorf("repository received %+v, want only the switch touched", repo.updated)
	}
}

// Pausing must take effect at once, so no network round trip belongs in this path.
func TestUpdateFeedNeverFetchesTheFeed(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}}}
	fetcher := &fakeFetcher{}

	if _, err := NewSubscriptionService(repo, fetcher).UpdateFeed(context.Background(), 1,
		domain.UpdateFeedInput{Enabled: ptr(false)}); err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if fetcher.calls != 0 {
		t.Errorf("fetcher called %d times, want 0 — the url has not changed", fetcher.calls)
	}
}

func TestUpdateFeedTrimsAndValidates(t *testing.T) {
	tests := []struct {
		name      string
		in        domain.UpdateFeedInput
		wantField string
	}{
		{"empty name", domain.UpdateFeedInput{Title: ptr("   ")}, "title"},
		{"negative cadence", domain.UpdateFeedInput{RefreshInterval: ptr(-time.Second)}, "refresh_interval_sec"},
		{"nothing at all", domain.UpdateFeedInput{}, "body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}}}

			_, err := updateService(repo).UpdateFeed(context.Background(), 1, tt.in)

			var ve domain.ValidationError
			if !errors.As(err, &ve) || ve.Field != tt.wantField {
				t.Fatalf("error = %v, want a %s ValidationError", err, tt.wantField)
			}
			if repo.updateCalls != 0 {
				t.Errorf("repository was asked to update %d times, want 0", repo.updateCalls)
			}
		})
	}
}

func TestUpdateFeedTrimsTheNewName(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}}}

	if _, err := updateService(repo).UpdateFeed(context.Background(), 1,
		domain.UpdateFeedInput{Title: ptr("  Datadog  ")}); err != nil {
		t.Fatalf("UpdateFeed() error = %v", err)
	}

	if repo.updated.Title == nil || *repo.updated.Title != "Datadog" {
		t.Errorf("repository received %v, want the trimmed name", repo.updated.Title)
	}
}

func TestUpdateFeedRejectsANameAlreadyTaken(t *testing.T) {
	repo := &fakeRepo{
		feeds:       []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}},
		titleExists: true,
	}

	_, err := updateService(repo).UpdateFeed(context.Background(), 1,
		domain.UpdateFeedInput{Title: ptr("Datadog")})

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("error = %v, want ErrDuplicateTitle", err)
	}
	if repo.updateCalls != 0 {
		t.Errorf("repository was asked to update %d times, want 0", repo.updateCalls)
	}
}

// Saving a feed under the name it already has is a no-op, not a collision with itself.
func TestUpdateFeedAllowsKeepingItsOwnName(t *testing.T) {
	repo := &fakeRepo{
		feeds:       []domain.Feed{{ID: 1, Title: "GitHub", Enabled: true}},
		titleExists: true,
	}

	if _, err := updateService(repo).UpdateFeed(context.Background(), 1,
		domain.UpdateFeedInput{Title: ptr("github")}); err != nil {
		t.Fatalf("UpdateFeed() error = %v, want its own name accepted", err)
	}

	if repo.updateCalls != 1 {
		t.Errorf("repository update calls = %d, want 1", repo.updateCalls)
	}
}

func TestUpdateFeedPropagatesAnUnknownFeed(t *testing.T) {
	repo := &fakeRepo{updateErr: fmt.Errorf("feed 99: %w", domain.ErrNotFound)}

	_, err := updateService(repo).UpdateFeed(context.Background(), 99,
		domain.UpdateFeedInput{Enabled: ptr(false)})

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}
