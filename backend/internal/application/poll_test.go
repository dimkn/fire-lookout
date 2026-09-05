package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

// pollRepo records what the poller asked of it. Separate from fakeRepo because the poller
// only touches the write side, and several assertions need per-feed detail.
type pollRepo struct {
	mu sync.Mutex

	due    []domain.Feed
	dueErr error

	saved     map[int64][]domain.StatusItem
	saveErr   error
	recorded  []domain.PollResult
	recordErr error
	pruned    map[int64]time.Time
	pruneErr  error
	gotNow    time.Time
}

func newPollRepo(due ...domain.Feed) *pollRepo {
	return &pollRepo{
		due:    due,
		saved:  map[int64][]domain.StatusItem{},
		pruned: map[int64]time.Time{},
	}
}

func (r *pollRepo) DueFeeds(_ context.Context, now time.Time) ([]domain.Feed, error) {
	r.gotNow = now
	return r.due, r.dueErr
}

func (r *pollRepo) SaveItems(_ context.Context, feedID int64, items []domain.StatusItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved[feedID] = items
	return nil
}

func (r *pollRepo) RecordPoll(_ context.Context, result domain.PollResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recorded = append(r.recorded, result)
	return r.recordErr
}

func (r *pollRepo) PruneItems(_ context.Context, feedID int64, before time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruned[feedID] = before
	return 0, r.pruneErr
}

// The read side is unused by the poller but required by the port.
func (r *pollRepo) ListFeeds(context.Context) ([]domain.Feed, error) { return nil, nil }

func (r *pollRepo) ListLatestItems(context.Context, time.Time) ([]domain.StatusItem, error) {
	return nil, nil
}

func (r *pollRepo) GetFeed(context.Context, int64) (domain.Feed, error) {
	return domain.Feed{}, domain.ErrNotFound
}

func (r *pollRepo) ListItems(context.Context, domain.ItemQuery) ([]domain.StatusItem, error) {
	return nil, nil
}
func (r *pollRepo) FeedExistsByURL(context.Context, string) (bool, error)   { return false, nil }
func (r *pollRepo) FeedExistsByTitle(context.Context, string) (bool, error) { return false, nil }
func (r *pollRepo) CreateFeed(_ context.Context, f domain.Feed) (domain.Feed, error) {
	return f, nil
}

func (r *pollRepo) outcomeFor(feedID int64) domain.PollResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, got := range r.recorded {
		if got.FeedID == feedID {
			return got
		}
	}
	return domain.PollResult{}
}

// pollFetcher answers per URL, so one run can mix successes and failures.
type pollFetcher struct {
	mu        sync.Mutex
	responses map[string]domain.FetchedFeed
	errs      map[string]error
	requests  []domain.FetchRequest
	inFlight  int
	maxSeen   int
	gate      chan struct{}
}

func newPollFetcher() *pollFetcher {
	return &pollFetcher{responses: map[string]domain.FetchedFeed{}, errs: map[string]error{}}
}

func (f *pollFetcher) Fetch(_ context.Context, req domain.FetchRequest) (domain.FetchedFeed, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.inFlight++
	if f.inFlight > f.maxSeen {
		f.maxSeen = f.inFlight
	}
	gate := f.gate
	f.mu.Unlock()

	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.inFlight--
	if err, ok := f.errs[req.URL]; ok {
		return domain.FetchedFeed{}, err
	}
	return f.responses[req.URL], nil
}

func (f *pollFetcher) requestFor(url string) (domain.FetchRequest, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, req := range f.requests {
		if req.URL == url {
			return req, true
		}
	}
	return domain.FetchRequest{}, false
}

func feed(id int64, url string) domain.Feed {
	return domain.Feed{ID: id, URL: url, Title: fmt.Sprintf("Feed %d", id), Enabled: true}
}

func fixedNow(t *testing.T, s string) func() time.Time {
	t.Helper()
	at := ts(s)
	return func() time.Time { return at }
}

func TestPollDueStoresEntriesAndDerivesTheirStatus(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	fetcher := newPollFetcher()
	fetcher.responses["https://a.test/feed.atom"] = domain.FetchedFeed{
		Title: "A Status",
		Items: []domain.StatusItem{
			{GUID: "i-1", Title: "Elevated errors", ContentText: "Investigating - looking into it."},
			{GUID: "i-2", Title: "Old thing", ContentText: "Resolved - all clear."},
		},
		ETag: `W/"abc"`, LastModified: "Mon, 24 Aug 2026 10:00:00 GMT",
	}

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.Updated != 1 {
		t.Errorf("summary.Updated = %d, want 1 (%+v)", summary.Updated, summary)
	}

	saved := repo.saved[1]
	if len(saved) != 2 {
		t.Fatalf("saved %d items, want 2", len(saved))
	}
	if saved[0].Status != domain.StatusInvestigating {
		t.Errorf("item 1 status = %q, want investigating", saved[0].Status)
	}
	if saved[1].Status != domain.StatusResolved {
		t.Errorf("item 2 status = %q, want resolved", saved[1].Status)
	}
	// The poller stamps when we saw each entry.
	if !saved[0].FetchedAt.Equal(ts("2026-08-24T12:00:00Z")) {
		t.Errorf("FetchedAt = %v, want the poll time", saved[0].FetchedAt)
	}
	if saved[0].FeedID != 1 {
		t.Errorf("FeedID = %d, want 1", saved[0].FeedID)
	}

	got := repo.outcomeFor(1)
	if got.Outcome != domain.PollUpdated {
		t.Errorf("outcome = %q, want updated", got.Outcome)
	}
	if got.ETag != `W/"abc"` || got.LastModified != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Errorf("validators = %q / %q, want them stored for next time", got.ETag, got.LastModified)
	}
}

func TestPollDueSendsTheStoredValidators(t *testing.T) {
	subject := feed(1, "https://a.test/feed.atom")
	subject.ETag = `W/"abc"`
	subject.LastModified = "Mon, 24 Aug 2026 10:00:00 GMT"
	repo := newPollRepo(subject)
	fetcher := newPollFetcher()

	if _, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background()); err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	req, ok := fetcher.requestFor("https://a.test/feed.atom")
	if !ok {
		t.Fatal("the feed was never fetched")
	}
	if req.ETag != `W/"abc"` || req.LastModified != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Errorf("request = %+v, want the stored validators", req)
	}
}

func TestPollDueTreatsNotModifiedAsASuccessfulPoll(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	fetcher := newPollFetcher()
	fetcher.responses["https://a.test/feed.atom"] = domain.FetchedFeed{
		NotModified: true, ETag: `W/"abc"`,
	}

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.NotModified != 1 {
		t.Errorf("summary.NotModified = %d, want 1", summary.NotModified)
	}
	if got := repo.outcomeFor(1).Outcome; got != domain.PollNotModified {
		t.Errorf("outcome = %q, want not_modified", got)
	}
	// Nothing changed, so nothing is written or pruned.
	if len(repo.saved) != 0 {
		t.Errorf("saved %v, want nothing stored for a 304", repo.saved)
	}
	if len(repo.pruned) != 0 {
		t.Errorf("pruned %v, want no pruning for a 304", repo.pruned)
	}
}

// 429 is not a failure: the attempt is recorded so the feed waits a full interval, and
// nothing is marked broken.
func TestPollDueTreatsRateLimitingAsASkip(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	fetcher := newPollFetcher()
	fetcher.errs["https://a.test/feed.atom"] = fmt.Errorf("get: %w", domain.ErrRateLimited)

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.RateLimited != 1 || summary.Failed != 0 {
		t.Errorf("summary = %+v, want one rate-limited and no failures", summary)
	}

	got := repo.outcomeFor(1)
	if got.Outcome != domain.PollRateLimited {
		t.Errorf("outcome = %q, want rate_limited", got.Outcome)
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want empty — a 429 is not a fault of the feed", got.Error)
	}
}

func TestPollDueRecordsAFailure(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	fetcher := newPollFetcher()
	fetcher.errs["https://a.test/feed.atom"] = fmt.Errorf("read: %w", domain.ErrFeedUnreachable)

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.Failed != 1 {
		t.Errorf("summary.Failed = %d, want 1", summary.Failed)
	}
	got := repo.outcomeFor(1)
	if got.Outcome != domain.PollFailed {
		t.Errorf("outcome = %q, want failed", got.Outcome)
	}
	if got.Error == "" {
		t.Error("Error is empty, want the reason recorded for the card to show")
	}
	if len(repo.saved) != 0 {
		t.Errorf("saved %v, want nothing stored for a failed poll", repo.saved)
	}
}

// One bad feed must not stop the others: each is recorded on its own.
func TestPollDueKeepsGoingAfterOneFeedFails(t *testing.T) {
	repo := newPollRepo(
		feed(1, "https://broken.test/feed.atom"),
		feed(2, "https://fine.test/feed.atom"),
		feed(3, "https://limited.test/feed.atom"),
	)
	fetcher := newPollFetcher()
	fetcher.errs["https://broken.test/feed.atom"] = fmt.Errorf("boom: %w", domain.ErrFeedUnreachable)
	fetcher.errs["https://limited.test/feed.atom"] = fmt.Errorf("slow down: %w", domain.ErrRateLimited)
	fetcher.responses["https://fine.test/feed.atom"] = domain.FetchedFeed{
		Title: "Fine",
		Items: []domain.StatusItem{{GUID: "x", ContentText: "Resolved - fine."}},
	}

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.Updated != 1 || summary.Failed != 1 || summary.RateLimited != 1 {
		t.Errorf("summary = %+v, want one of each", summary)
	}
	if len(repo.recorded) != 3 {
		t.Errorf("recorded %d outcomes, want one per feed", len(repo.recorded))
	}
	if got := repo.outcomeFor(2).Outcome; got != domain.PollUpdated {
		t.Errorf("healthy feed outcome = %q, want updated", got)
	}
}

func TestPollDuePrunesOldItemsAfterASuccessfulPoll(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	fetcher := newPollFetcher()
	fetcher.responses["https://a.test/feed.atom"] = domain.FetchedFeed{
		Items: []domain.StatusItem{{GUID: "x", ContentText: "Resolved - ok."}},
	}
	now := ts("2026-08-24T12:00:00Z")

	if _, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background()); err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	before, ok := repo.pruned[1]
	if !ok {
		t.Fatal("nothing was pruned, want the retention window applied")
	}
	if want := now.Add(-domain.RetentionWindow); !before.Equal(want) {
		t.Errorf("pruned before %v, want %v", before, want)
	}
}

func TestPollDueWithNothingDue(t *testing.T) {
	repo := newPollRepo()
	fetcher := newPollFetcher()

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary != (Summary{}) {
		t.Errorf("summary = %+v, want everything zero", summary)
	}
	if len(fetcher.requests) != 0 {
		t.Errorf("fetched %d feeds, want none", len(fetcher.requests))
	}
}

func TestPollDueAsksForWhatIsDueNow(t *testing.T) {
	repo := newPollRepo()

	if _, err := NewPollService(repo, newPollFetcher(), fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background()); err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if !repo.gotNow.Equal(ts("2026-08-24T12:00:00Z")) {
		t.Errorf("asked for feeds due at %v, want the current time", repo.gotNow)
	}
}

func TestPollDueFailsWhenTheDueQueryFails(t *testing.T) {
	boom := errors.New("boom")
	repo := newPollRepo()
	repo.dueErr = boom

	_, err := NewPollService(repo, newPollFetcher(), fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())

	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to wrap %v", err, boom)
	}
}

// A storage failure on one feed is recorded as that feed's failure, not a crash.
func TestPollDueReportsAStorageFailureAsAFeedFailure(t *testing.T) {
	repo := newPollRepo(feed(1, "https://a.test/feed.atom"))
	repo.saveErr = errors.New("disk full")
	fetcher := newPollFetcher()
	fetcher.responses["https://a.test/feed.atom"] = domain.FetchedFeed{
		Items: []domain.StatusItem{{GUID: "x"}},
	}

	summary, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background())
	if err != nil {
		t.Fatalf("PollDue() error = %v", err)
	}

	if summary.Failed != 1 {
		t.Errorf("summary = %+v, want the feed counted as failed", summary)
	}
	if got := repo.outcomeFor(1).Outcome; got != domain.PollFailed {
		t.Errorf("outcome = %q, want failed", got)
	}
}

func TestPollDueLimitsConcurrency(t *testing.T) {
	feeds := make([]domain.Feed, 0, 12)
	fetcher := newPollFetcher()
	fetcher.gate = make(chan struct{})
	for i := int64(1); i <= 12; i++ {
		url := fmt.Sprintf("https://feed-%d.test/atom", i)
		feeds = append(feeds, feed(i, url))
		fetcher.responses[url] = domain.FetchedFeed{NotModified: true}
	}
	repo := newPollRepo(feeds...)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := NewPollService(repo, fetcher, fixedNow(t, "2026-08-24T12:00:00Z")).PollDue(context.Background()); err != nil {
			t.Errorf("PollDue() error = %v", err)
		}
	}()

	// Let everything through, then check how many were ever in flight at once.
	close(fetcher.gate)
	<-done

	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	if fetcher.maxSeen > maxConcurrentPolls {
		t.Errorf("%d fetches ran at once, want at most %d", fetcher.maxSeen, maxConcurrentPolls)
	}
	if len(fetcher.requests) != 12 {
		t.Errorf("fetched %d feeds, want all 12", len(fetcher.requests))
	}
}
