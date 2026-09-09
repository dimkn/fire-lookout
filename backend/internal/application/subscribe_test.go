package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

type fakeFetcher struct {
	title string
	err   error

	calls  int
	gotURL string
}

func (f *fakeFetcher) Fetch(_ context.Context, req domain.FetchRequest) (domain.FetchedFeed, error) {
	f.calls++
	f.gotURL = req.URL
	if f.err != nil {
		return domain.FetchedFeed{}, f.err
	}
	return domain.FetchedFeed{Title: f.title}, nil
}

func goodInput() domain.SubscribeInput {
	return domain.SubscribeInput{URL: "https://www.githubstatus.com/history.atom", Title: "GitHub"}
}

func TestSubscribeFeedStoresASanitizedFeed(t *testing.T) {
	repo := &fakeRepo{}
	fetcher := &fakeFetcher{title: "GitHub Status"}

	got, err := NewSubscriptionService(repo, fetcher).SubscribeFeed(context.Background(),
		domain.SubscribeInput{URL: "  https://a.test/feed.atom ", Title: "  GitHub  "})
	if err != nil {
		t.Fatalf("SubscribeFeed() error = %v", err)
	}

	if got.Title != "GitHub" || got.URL != "https://a.test/feed.atom" {
		t.Errorf("stored feed = %+v, want trimmed title and url", got)
	}
	if repo.created.Title != "GitHub" {
		t.Errorf("repository received title %q, want the trimmed one", repo.created.Title)
	}
	if repo.created.GroupID != domain.UngroupedID {
		t.Errorf("GroupID = %d, want ungrouped", repo.created.GroupID)
	}
	if !repo.created.Enabled {
		t.Error("Enabled = false, want true by default")
	}
	if repo.created.RefreshInterval != domain.DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want the default", repo.created.RefreshInterval)
	}
}

// The validation fetch proves the endpoint is a feed; it is not a poll, and it ingests no
// incidents. Claiming otherwise would paint a brand-new system green.
func TestSubscribeFeedLeavesHealthUnset(t *testing.T) {
	repo := &fakeRepo{}

	if _, err := NewSubscriptionService(repo, &fakeFetcher{}).SubscribeFeed(context.Background(), goodInput()); err != nil {
		t.Fatalf("SubscribeFeed() error = %v", err)
	}

	if repo.created.LastFetchedAt != nil || repo.created.LastSuccessAt != nil {
		t.Errorf("health = %v / %v, want both nil so the light stays unknown",
			repo.created.LastFetchedAt, repo.created.LastSuccessAt)
	}
	if repo.created.LastError != "" {
		t.Errorf("LastError = %q, want empty", repo.created.LastError)
	}
}

func TestSubscribeFeedValidatesBeforeAnythingElse(t *testing.T) {
	repo := &fakeRepo{}
	fetcher := &fakeFetcher{}

	_, err := NewSubscriptionService(repo, fetcher).SubscribeFeed(context.Background(),
		domain.SubscribeInput{URL: "https://a.test/feed.atom", Title: "   "})

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if fetcher.calls != 0 {
		t.Errorf("fetcher called %d times, want 0 for invalid input", fetcher.calls)
	}
	if repo.createCalls != 0 {
		t.Errorf("repository asked to create %d times, want 0", repo.createCalls)
	}
}

func TestSubscribeFeedRejectsADuplicateURL(t *testing.T) {
	repo := &fakeRepo{urlExists: true}
	fetcher := &fakeFetcher{}

	_, err := NewSubscriptionService(repo, fetcher).SubscribeFeed(context.Background(), goodInput())

	if !errors.Is(err, domain.ErrDuplicateURL) {
		t.Fatalf("error = %v, want ErrDuplicateURL", err)
	}
	if !errors.Is(err, domain.ErrConflict) {
		t.Error("error should also match the ErrConflict category")
	}
	// No point paying for a network round trip on input we already know we will reject.
	if fetcher.calls != 0 {
		t.Errorf("fetcher called %d times, want 0", fetcher.calls)
	}
	if repo.createCalls != 0 {
		t.Errorf("repository asked to create %d times, want 0", repo.createCalls)
	}
}

func TestSubscribeFeedRejectsADuplicateName(t *testing.T) {
	repo := &fakeRepo{titleExists: true}
	fetcher := &fakeFetcher{}

	_, err := NewSubscriptionService(repo, fetcher).SubscribeFeed(context.Background(), goodInput())

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("error = %v, want ErrDuplicateTitle", err)
	}
	if fetcher.calls != 0 {
		t.Errorf("fetcher called %d times, want 0", fetcher.calls)
	}
	if repo.createCalls != 0 {
		t.Errorf("repository asked to create %d times, want 0", repo.createCalls)
	}
}

func TestSubscribeFeedRejectsAnUnreadableFeed(t *testing.T) {
	repo := &fakeRepo{}
	fetcher := &fakeFetcher{err: fmt.Errorf("get: %w", domain.ErrFeedUnreachable)}

	_, err := NewSubscriptionService(repo, fetcher).SubscribeFeed(context.Background(), goodInput())

	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Fatalf("error = %v, want ErrFeedUnreachable", err)
	}
	// Nothing may be stored when the endpoint turns out not to be a feed.
	if repo.createCalls != 0 {
		t.Errorf("repository asked to create %d times, want 0", repo.createCalls)
	}
}

func TestSubscribeFeedChecksTheURLItWasGiven(t *testing.T) {
	fetcher := &fakeFetcher{}

	if _, err := NewSubscriptionService(&fakeRepo{}, fetcher).SubscribeFeed(context.Background(),
		domain.SubscribeInput{URL: "  https://a.test/feed.atom  ", Title: "GitHub"}); err != nil {
		t.Fatalf("SubscribeFeed() error = %v", err)
	}

	if fetcher.gotURL != "https://a.test/feed.atom" {
		t.Errorf("fetched %q, want the trimmed url", fetcher.gotURL)
	}
}

// The unique indexes are the real guard; a race that slips past the pre-checks must still
// surface as a conflict rather than a 500.
func TestSubscribeFeedPropagatesAStorageConflict(t *testing.T) {
	repo := &fakeRepo{createErr: fmt.Errorf("insert: %w", domain.ErrDuplicateTitle)}

	_, err := NewSubscriptionService(repo, &fakeFetcher{}).SubscribeFeed(context.Background(), goodInput())

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("error = %v, want ErrDuplicateTitle", err)
	}
}

func TestSubscribeFeedPropagatesLookupFailures(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		repo *fakeRepo
	}{
		{"url lookup fails", &fakeRepo{urlExistsErr: boom}},
		{"title lookup fails", &fakeRepo{titleExistsErr: boom}},
		{"insert fails", &fakeRepo{createErr: boom}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSubscriptionService(tt.repo, &fakeFetcher{}).SubscribeFeed(context.Background(), goodInput())

			if !errors.Is(err, boom) {
				t.Errorf("error = %v, want it to wrap %v", err, boom)
			}
		})
	}
}

func TestSubscribeFeedHonoursExplicitCadence(t *testing.T) {
	repo := &fakeRepo{}
	in := goodInput()
	in.RefreshInterval = 90 * time.Second

	if _, err := NewSubscriptionService(repo, &fakeFetcher{}).SubscribeFeed(context.Background(), in); err != nil {
		t.Fatalf("SubscribeFeed() error = %v", err)
	}

	if repo.created.RefreshInterval != 90*time.Second {
		t.Errorf("RefreshInterval = %v, want 90s", repo.created.RefreshInterval)
	}
}
