package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"fire-lookout/backend/internal/domain"
)

// maxConcurrentPolls bounds how many feeds are fetched at once. Providers are third parties
// and this is a local tool: a handful in flight is plenty, and it keeps one slow endpoint
// from starving the rest.
const maxConcurrentPolls = 4

// Summary counts what a poll run amounted to, for the scheduler to log.
type Summary struct {
	Updated     int
	NotModified int
	RateLimited int
	Failed      int
}

// Total reports how many feeds were attempted.
func (s Summary) Total() int {
	return s.Updated + s.NotModified + s.RateLimited + s.Failed
}

// PollService reads the feeds that are due and stores what they say.
type PollService struct {
	repo    domain.StatusRepository
	fetcher domain.FeedFetcher
	now     func() time.Time
}

// NewPollService wires the service to its ports. now is injected so tests can pin the clock.
func NewPollService(repo domain.StatusRepository, fetcher domain.FeedFetcher, now func() time.Time) *PollService {
	if now == nil {
		now = time.Now
	}
	return &PollService{repo: repo, fetcher: fetcher, now: now}
}

// PollDue fetches every feed whose cadence has elapsed, bounded to maxConcurrentPolls at a
// time. Each feed's outcome is recorded independently: one unreachable endpoint must not stop
// the others, so only a failure to *find* the due feeds aborts the run.
func (s *PollService) PollDue(ctx context.Context) (Summary, error) {
	at := s.now().UTC().Truncate(time.Second)

	due, err := s.repo.DueFeeds(ctx, at)
	if err != nil {
		return Summary{}, fmt.Errorf("list due feeds: %w", err)
	}

	var (
		mu      sync.Mutex
		summary Summary
		wg      sync.WaitGroup
	)
	slots := make(chan struct{}, maxConcurrentPolls)

	for _, f := range due {
		wg.Add(1)
		go func(f domain.Feed) {
			defer wg.Done()

			slots <- struct{}{}
			defer func() { <-slots }()

			outcome := s.pollOne(ctx, f, at)

			mu.Lock()
			defer mu.Unlock()
			switch outcome {
			case domain.PollUpdated:
				summary.Updated++
			case domain.PollNotModified:
				summary.NotModified++
			case domain.PollRateLimited:
				summary.RateLimited++
			case domain.PollFailed:
				summary.Failed++
			}
		}(f)
	}
	wg.Wait()

	return summary, nil
}

// pollOne fetches a single feed and records what happened, returning the outcome it recorded.
func (s *PollService) pollOne(ctx context.Context, f domain.Feed, at time.Time) domain.PollOutcome {
	fetched, err := s.fetcher.Fetch(ctx, domain.FetchRequest{
		URL:          f.URL,
		ETag:         f.ETag,
		LastModified: f.LastModified,
	})

	switch {
	case errors.Is(err, domain.ErrRateLimited):
		// Not a fault of the feed: record the attempt so it waits a full interval, and leave
		// last_error and last_success_at untouched.
		return s.record(ctx, domain.PollResult{
			FeedID: f.ID, At: at, Outcome: domain.PollRateLimited,
			ETag: f.ETag, LastModified: f.LastModified,
		})

	case err != nil:
		return s.record(ctx, domain.PollResult{
			FeedID: f.ID, At: at, Outcome: domain.PollFailed, Error: err.Error(),
		})

	case fetched.NotModified:
		// A 304 is a successful poll: we asked and learned that nothing changed.
		return s.record(ctx, domain.PollResult{
			FeedID: f.ID, At: at, Outcome: domain.PollNotModified,
			ETag: validator(fetched.ETag, f.ETag), LastModified: validator(fetched.LastModified, f.LastModified),
		})
	}

	items := make([]domain.StatusItem, 0, len(fetched.Items))
	for _, item := range fetched.Items {
		item.FeedID = f.ID
		item.FetchedAt = at
		// Interpreting provider prose is a domain rule, applied here rather than in the
		// adapter so the use case is where you can see it happen.
		item.Status = domain.ParseStatus(item.Title, statusText(item))
		items = append(items, item)
	}

	if err := s.repo.SaveItems(ctx, f.ID, items); err != nil {
		return s.record(ctx, domain.PollResult{
			FeedID: f.ID, At: at, Outcome: domain.PollFailed,
			Error: fmt.Errorf("store items: %w", err).Error(),
		})
	}

	outcome := s.record(ctx, domain.PollResult{
		FeedID: f.ID, At: at, Outcome: domain.PollUpdated,
		ETag: fetched.ETag, LastModified: fetched.LastModified,
	})

	// Retention: the dashboard only ever shows a recent window, so old entries are dropped
	// once we know the feed is healthy. A pruning failure does not undo a good poll.
	if _, err := s.repo.PruneItems(ctx, f.ID, at.Add(-domain.RetentionWindow)); err != nil {
		return outcome
	}
	return outcome
}

// record writes the bookkeeping and reports the outcome it wrote, so callers can count it
// even when the write itself fails (there is nothing better to do about that here).
func (s *PollService) record(ctx context.Context, result domain.PollResult) domain.PollOutcome {
	_ = s.repo.RecordPoll(ctx, result)
	return result.Outcome
}

// statusText prefers the stripped text, falling back to the raw body for feeds that only
// carry HTML.
func statusText(item domain.StatusItem) string {
	if item.ContentText != "" {
		return item.ContentText
	}
	return item.ContentHTML
}

// validator keeps whatever the provider just sent, falling back to what we already had: a
// 304 often carries no ETag of its own.
func validator(fresh, stored string) string {
	if fresh != "" {
		return fresh
	}
	return stored
}
