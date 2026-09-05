package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

func newFeed(title, url string) domain.Feed {
	return domain.Feed{
		URL:             url,
		Title:           title,
		GroupID:         domain.UngroupedID,
		Enabled:         true,
		RefreshInterval: domain.DefaultRefreshInterval,
	}
}

func TestCreateFeedStoresAndReturnsTheRow(t *testing.T) {
	repo, _ := newTestRepo(t)
	before := time.Now().UTC().Add(-time.Second)

	got, err := repo.CreateFeed(context.Background(), newFeed("GitHub", "https://a.test/feed.atom"))
	if err != nil {
		t.Fatalf("CreateFeed() error = %v", err)
	}

	if got.ID == 0 {
		t.Error("ID = 0, want the id the database assigned")
	}
	if got.Title != "GitHub" || got.URL != "https://a.test/feed.atom" {
		t.Errorf("feed = %+v, want the values we passed", got)
	}
	if got.CreatedAt.Before(before) || got.CreatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("CreatedAt = %v, want roughly now", got.CreatedAt)
	}
	if !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Errorf("UpdatedAt = %v, want it to match CreatedAt %v", got.UpdatedAt, got.CreatedAt)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}

	// It must come back out of a fresh read the same way.
	feeds, err := repo.ListFeeds(context.Background())
	if err != nil {
		t.Fatalf("ListFeeds() error = %v", err)
	}
	if len(feeds) != 1 || feeds[0].Title != "GitHub" {
		t.Fatalf("stored feeds = %+v, want the one we created", feeds)
	}
	if !feeds[0].Enabled {
		t.Error("Enabled = false, want true as stored")
	}
	if feeds[0].RefreshInterval != domain.DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want %v", feeds[0].RefreshInterval, domain.DefaultRefreshInterval)
	}
	if feeds[0].LastFetchedAt != nil || feeds[0].LastSuccessAt != nil || feeds[0].LastError != "" {
		t.Error("health columns are set, want them all empty for a brand-new feed")
	}
}

func TestCreateFeedRejectsADuplicateURL(t *testing.T) {
	repo, _ := newTestRepo(t)
	if _, err := repo.CreateFeed(context.Background(), newFeed("GitHub", "https://a.test/feed.atom")); err != nil {
		t.Fatalf("first CreateFeed() error = %v", err)
	}

	_, err := repo.CreateFeed(context.Background(), newFeed("Something else", "https://a.test/feed.atom"))

	if !errors.Is(err, domain.ErrDuplicateURL) {
		t.Fatalf("error = %v, want ErrDuplicateURL", err)
	}
	if !errors.Is(err, domain.ErrConflict) {
		t.Error("error should also match the ErrConflict category")
	}
}

func TestCreateFeedRejectsADuplicateTitle(t *testing.T) {
	repo, _ := newTestRepo(t)
	if _, err := repo.CreateFeed(context.Background(), newFeed("GitHub", "https://a.test/feed.atom")); err != nil {
		t.Fatalf("first CreateFeed() error = %v", err)
	}

	_, err := repo.CreateFeed(context.Background(), newFeed("GitHub", "https://b.test/feed.atom"))

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("error = %v, want ErrDuplicateTitle", err)
	}
}

// The index is case-insensitive, because "GitHub" and "github" are the same name to a
// person reading the dashboard.
func TestCreateFeedRejectsATitleDifferingOnlyInCase(t *testing.T) {
	repo, _ := newTestRepo(t)
	if _, err := repo.CreateFeed(context.Background(), newFeed("GitHub", "https://a.test/feed.atom")); err != nil {
		t.Fatalf("first CreateFeed() error = %v", err)
	}

	_, err := repo.CreateFeed(context.Background(), newFeed("gItHuB", "https://b.test/feed.atom"))

	if !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("error = %v, want ErrDuplicateTitle", err)
	}
}

func TestFeedExistsByURL(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed.atom", "", "", "")

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"stored url", "https://a.test/feed.atom", true},
		{"unknown url", "https://b.test/feed.atom", false},
		// Urls are compared exactly: paths are case-sensitive in general.
		{"different case", "https://A.test/feed.atom", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FeedExistsByURL(context.Background(), tt.url)
			if err != nil {
				t.Fatalf("FeedExistsByURL() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("FeedExistsByURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestFeedExistsByTitle(t *testing.T) {
	repo, db := newTestRepo(t)
	insertFeed(t, db, 1, "GitHub", "https://a.test/feed.atom", "", "", "")

	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"exact name", "GitHub", true},
		{"different case", "github", true},
		{"shouty", "GITHUB", true},
		{"unknown name", "Datadog", false},
		// A prefix is a different name, not a match.
		{"prefix only", "Git", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.FeedExistsByTitle(context.Background(), tt.title)
			if err != nil {
				t.Fatalf("FeedExistsByTitle() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("FeedExistsByTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestCreateFeedThenReadItBack(t *testing.T) {
	repo, _ := newTestRepo(t)

	created, err := repo.CreateFeed(context.Background(), newFeed("Datadog", "https://dd.test/feed.atom"))
	if err != nil {
		t.Fatalf("CreateFeed() error = %v", err)
	}

	got, err := repo.GetFeed(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetFeed() error = %v", err)
	}
	if got.Title != "Datadog" {
		t.Errorf("Title = %q, want Datadog", got.Title)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %v, want the value CreateFeed reported (%v)", got.CreatedAt, created.CreatedAt)
	}
}
