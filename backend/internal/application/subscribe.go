package application

import (
	"context"
	"fmt"

	"fire-lookout/backend/internal/domain"
)

// SubscriptionService is the write side: adding a feed to the status page.
type SubscriptionService struct {
	repo    domain.StatusRepository
	fetcher domain.FeedFetcher
}

// NewSubscriptionService wires the service to its ports.
func NewSubscriptionService(repo domain.StatusRepository, fetcher domain.FeedFetcher) *SubscriptionService {
	return &SubscriptionService{repo: repo, fetcher: fetcher}
}

// SubscribeFeed validates the input, proves the endpoint really is a feed, and stores it.
//
// The order is deliberate and cheapest-first: sanitise, then the two duplicate lookups,
// and only then the network round trip — there is no point fetching a URL we already know
// we will reject. Nothing is written unless every check passes.
//
// The feed's health columns are left unset. Fetching the endpoint proves it parses; it is
// not a poll and stores no incidents, so the new system shows an unknown (grey) light
// until the poller has actually read it.
func (s *SubscriptionService) SubscribeFeed(ctx context.Context, in domain.SubscribeInput) (domain.Feed, error) {
	input, err := in.Sanitize()
	if err != nil {
		return domain.Feed{}, err
	}

	taken, err := s.repo.FeedExistsByURL(ctx, input.URL)
	if err != nil {
		return domain.Feed{}, fmt.Errorf("look up feed url: %w", err)
	}
	if taken {
		return domain.Feed{}, fmt.Errorf("subscribe %q: %w", input.URL, domain.ErrDuplicateURL)
	}

	taken, err = s.repo.FeedExistsByTitle(ctx, input.Title)
	if err != nil {
		return domain.Feed{}, fmt.Errorf("look up feed title: %w", err)
	}
	if taken {
		return domain.Feed{}, fmt.Errorf("subscribe %q: %w", input.Title, domain.ErrDuplicateTitle)
	}

	if _, err := s.fetcher.Fetch(ctx, input.URL); err != nil {
		return domain.Feed{}, fmt.Errorf("validate feed %q: %w", input.URL, err)
	}

	feed := domain.Feed{
		URL:             input.URL,
		Title:           input.Title,
		GroupID:         input.GroupID,
		Enabled:         *input.Enabled,
		RefreshInterval: input.RefreshInterval,
	}

	created, err := s.repo.CreateFeed(ctx, feed)
	if err != nil {
		return domain.Feed{}, fmt.Errorf("create feed: %w", err)
	}
	return created, nil
}
