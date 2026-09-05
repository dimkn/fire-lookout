package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

// fakeRepo is an in-memory StatusRepository. It also records what it was asked for, so
// tests can assert on clamping and pass-through.
type fakeRepo struct {
	feeds  []domain.Feed
	latest []domain.StatusItem
	items  []domain.StatusItem

	feedsErr  error
	latestErr error
	getErr    error
	itemsErr  error

	gotFeedID int64
	gotSince  *time.Time
	gotLimit  int
	itemsCall int
	gotAsOf   time.Time
	gotQuery  domain.ItemQuery

	// Write side, exercised by the subscribe use case.
	urlExists      bool
	titleExists    bool
	urlExistsErr   error
	titleExistsErr error
	createErr      error
	created        domain.Feed
	createCalls    int
	gotURLLookup   string
	gotTitleLookup string
}

func (r *fakeRepo) FeedExistsByURL(_ context.Context, url string) (bool, error) {
	r.gotURLLookup = url
	return r.urlExists, r.urlExistsErr
}

func (r *fakeRepo) FeedExistsByTitle(_ context.Context, title string) (bool, error) {
	r.gotTitleLookup = title
	return r.titleExists, r.titleExistsErr
}

// The poller's methods; the read-side tests do not exercise them (see poll_test.go).
func (r *fakeRepo) DueFeeds(context.Context, time.Time) ([]domain.Feed, error) {
	return nil, nil
}

func (r *fakeRepo) SaveItems(context.Context, int64, []domain.StatusItem) error { return nil }

func (r *fakeRepo) RecordPoll(context.Context, domain.PollResult) error { return nil }

func (r *fakeRepo) PruneItems(context.Context, int64, time.Time) (int64, error) { return 0, nil }

func (r *fakeRepo) CreateFeed(_ context.Context, feed domain.Feed) (domain.Feed, error) {
	r.createCalls++
	if r.createErr != nil {
		return domain.Feed{}, r.createErr
	}
	r.created = feed
	stored := feed
	stored.ID = 42
	stored.CreatedAt = ts("2026-08-24T12:00:00Z")
	stored.UpdatedAt = stored.CreatedAt
	return stored, nil
}

func (r *fakeRepo) ListFeeds(context.Context) ([]domain.Feed, error) {
	return r.feeds, r.feedsErr
}

func (r *fakeRepo) ListLatestItems(_ context.Context, asOf time.Time) ([]domain.StatusItem, error) {
	r.gotAsOf = asOf
	return r.latest, r.latestErr
}

func (r *fakeRepo) GetFeed(_ context.Context, id int64) (domain.Feed, error) {
	if r.getErr != nil {
		return domain.Feed{}, r.getErr
	}
	for _, f := range r.feeds {
		if f.ID == id {
			return f, nil
		}
	}
	return domain.Feed{}, fmt.Errorf("feed %d: %w", id, domain.ErrNotFound)
}

func (r *fakeRepo) ListItems(_ context.Context, q domain.ItemQuery) ([]domain.StatusItem, error) {
	r.gotQuery = q
	r.gotFeedID, r.gotSince, r.gotLimit = q.FeedID, q.Since, q.Limit
	r.itemsCall++
	return r.items, r.itemsErr
}

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

// nowForReads is the clock the read-side tests pin, so "the future" is unambiguous.
const nowForReads = "2026-08-24T12:00:00Z"

// The overview must ask for entries as of now: anything dated later is an announcement of
// work still to come, not the system's current state.
func TestOverviewAsksForEntriesAsOfNow(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}}

	if _, err := NewStatusService(repo, fixedNow(t, nowForReads)).Overview(context.Background()); err != nil {
		t.Fatalf("Overview() error = %v", err)
	}

	if !repo.gotAsOf.Equal(ts(nowForReads)) {
		t.Errorf("asked for entries as of %v, want %v", repo.gotAsOf, ts(nowForReads))
	}
}

// A feed whose only entries are future-dated maintenance windows has said nothing about
// now, so it falls through to the polled-successfully rule: operational, not degraded.
// This is the Cloudflare case.
func TestOverviewIgnoresFeedsWhoseOnlyEntriesAreStillToCome(t *testing.T) {
	repo := &fakeRepo{
		feeds: []domain.Feed{{ID: 1, Title: "Cloudflare", LastSuccessAt: ptr(ts(nowForReads))}},
		// The repository already applied the as-of filter, so nothing comes back.
		latest: nil,
	}

	got, err := NewStatusService(repo, fixedNow(t, nowForReads)).Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}

	if got[0].Indicator != domain.IndicatorOperational {
		t.Errorf("indicator = %q, want operational", got[0].Indicator)
	}
	if got[0].LastUpdatedAt != nil {
		t.Errorf("last updated = %v, want nil rather than a date in the future", got[0].LastUpdatedAt)
	}
}

func TestFeedItemsAsksForEntriesAsOfNow(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}}

	if _, err := NewStatusService(repo, fixedNow(t, nowForReads)).FeedItems(context.Background(), 1, nil, 10); err != nil {
		t.Fatalf("FeedItems() error = %v", err)
	}

	if !repo.gotQuery.AsOf.Equal(ts(nowForReads)) {
		t.Errorf("query AsOf = %v, want %v", repo.gotQuery.AsOf, ts(nowForReads))
	}
	if repo.gotQuery.FeedID != 1 || repo.gotQuery.Limit != 10 {
		t.Errorf("query = %+v, want the feed and limit carried through", repo.gotQuery)
	}
}

func TestOverviewAssemblesOneRowPerFeed(t *testing.T) {
	repo := &fakeRepo{
		feeds: []domain.Feed{
			{ID: 1, Title: "GitHub", LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z"))},
			{ID: 2, Title: "Datadog", LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z"))},
			{ID: 3, Title: "Never polled"},
		},
		latest: []domain.StatusItem{
			{ID: 10, FeedID: 1, Status: domain.StatusInvestigating, PublishedAt: ts("2026-08-18T08:00:00Z")},
		},
	}

	got, err := NewStatusService(repo, fixedNow(t, nowForReads)).Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}

	byID := map[int64]domain.SystemOverview{}
	for _, o := range got {
		byID[o.Feed.ID] = o
	}
	if want := domain.IndicatorOutage; byID[1].Indicator != want {
		t.Errorf("feed 1 indicator = %q, want %q", byID[1].Indicator, want)
	}
	if want := domain.IndicatorOperational; byID[2].Indicator != want {
		t.Errorf("feed 2 indicator = %q, want %q", byID[2].Indicator, want)
	}
	if want := domain.IndicatorUnknown; byID[3].Indicator != want {
		t.Errorf("feed 3 indicator = %q, want %q", byID[3].Indicator, want)
	}
	if byID[1].CurrentStatus == nil || *byID[1].CurrentStatus != domain.StatusInvestigating {
		t.Errorf("feed 1 current status = %v, want investigating", byID[1].CurrentStatus)
	}
	if byID[2].CurrentStatus != nil {
		t.Errorf("feed 2 current status = %v, want nil", *byID[2].CurrentStatus)
	}
}

func TestOverviewOrdersByTitleCaseInsensitivelyThenID(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{
		{ID: 4, Title: "zulip"},
		{ID: 1, Title: "Datadog"},
		{ID: 3, Title: "aws"},
		{ID: 2, Title: "aws"}, // same title: lower id wins
	}}

	got, err := NewStatusService(repo, fixedNow(t, nowForReads)).Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}

	want := []int64{2, 3, 1, 4}
	for i, id := range want {
		if got[i].Feed.ID != id {
			ids := make([]int64, len(got))
			for j, o := range got {
				ids[j] = o.Feed.ID
			}
			t.Fatalf("order = %v, want %v", ids, want)
		}
	}
}

func TestOverviewWithNoFeedsIsEmpty(t *testing.T) {
	got, err := NewStatusService(&fakeRepo{}, fixedNow(t, nowForReads)).Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d rows, want none", len(got))
	}
}

func TestOverviewPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	tests := []struct {
		name string
		repo *fakeRepo
	}{
		{"listing feeds fails", &fakeRepo{feedsErr: boom}},
		{"listing latest items fails", &fakeRepo{latestErr: boom}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStatusService(tt.repo, fixedNow(t, nowForReads)).Overview(context.Background())
			if !errors.Is(err, boom) {
				t.Errorf("error = %v, want it to wrap %v", err, boom)
			}
		})
	}
}

func TestFeedItemsClampsLimit(t *testing.T) {
	tests := []struct {
		name string
		ask  int
		want int
	}{
		{"zero means default", 0, domain.DefaultItemLimit},
		{"negative means default", -7, domain.DefaultItemLimit},
		{"within bounds is honoured", 10, 10},
		{"at the cap is honoured", domain.MaxItemLimit, domain.MaxItemLimit},
		{"above the cap is clamped", domain.MaxItemLimit + 500, domain.MaxItemLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}}

			if _, err := NewStatusService(repo, fixedNow(t, nowForReads)).FeedItems(context.Background(), 1, nil, tt.ask); err != nil {
				t.Fatalf("FeedItems() error = %v", err)
			}
			if repo.gotLimit != tt.want {
				t.Errorf("limit passed to repository = %d, want %d", repo.gotLimit, tt.want)
			}
		})
	}
}

func TestFeedItemsPassesSinceThrough(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}}
	since := ts("2026-08-11T00:00:00Z")

	if _, err := NewStatusService(repo, fixedNow(t, nowForReads)).FeedItems(context.Background(), 1, &since, 5); err != nil {
		t.Fatalf("FeedItems() error = %v", err)
	}

	if repo.gotFeedID != 1 {
		t.Errorf("feed id passed to repository = %d, want 1", repo.gotFeedID)
	}
	if repo.gotSince == nil || !repo.gotSince.Equal(since) {
		t.Errorf("since passed to repository = %v, want %v", repo.gotSince, since)
	}
}

func TestFeedItemsUnknownFeedIsNotFound(t *testing.T) {
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}}

	_, err := NewStatusService(repo, fixedNow(t, nowForReads)).FeedItems(context.Background(), 99, nil, 0)

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want it to wrap ErrNotFound", err)
	}
	if repo.itemsCall != 0 {
		t.Errorf("repository was asked for items %d times, want 0", repo.itemsCall)
	}
}

func TestFeedItemsReturnsWhatTheRepositoryHas(t *testing.T) {
	items := []domain.StatusItem{
		{ID: 2, FeedID: 1, Title: "Newer", PublishedAt: ts("2026-08-18T08:00:00Z")},
		{ID: 1, FeedID: 1, Title: "Older", PublishedAt: ts("2026-08-17T08:00:00Z")},
	}
	repo := &fakeRepo{feeds: []domain.Feed{{ID: 1, Title: "GitHub"}}, items: items}

	got, err := NewStatusService(repo, fixedNow(t, nowForReads)).FeedItems(context.Background(), 1, nil, 0)
	if err != nil {
		t.Fatalf("FeedItems() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Errorf("got %+v, want the repository's order preserved", got)
	}
}
