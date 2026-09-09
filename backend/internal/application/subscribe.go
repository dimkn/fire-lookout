package application

import (
	"context"
	"fmt"
	"strings"

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

	// No validators: this is the first time we have ever looked at this URL.
	if _, err := s.fetcher.Fetch(ctx, domain.FetchRequest{URL: input.URL}); err != nil {
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

// UpdateFeed applies a partial update — the pause switch, the name, the cadence, the group.
//
// Unlike subscribing, the feed's URL cannot change, so there is nothing to re-validate over
// the network: no fetch happens here. Pausing therefore takes effect immediately, which is
// the point of a switch.
func (s *SubscriptionService) UpdateFeed(ctx context.Context, id int64, in domain.UpdateFeedInput) (domain.Feed, error) {
	input, err := in.Sanitize()
	if err != nil {
		return domain.Feed{}, err
	}
	if input.IsEmpty() {
		return domain.Feed{}, domain.ValidationError{
			Field:   "body",
			Message: "Nothing to update.",
		}
	}

	// The unique index is the real guard; checking first turns the common case into a clear
	// conflict instead of a constraint violation.
	if input.Title != nil {
		taken, err := s.repo.FeedExistsByTitle(ctx, *input.Title)
		if err != nil {
			return domain.Feed{}, fmt.Errorf("look up feed title: %w", err)
		}
		if taken {
			current, err := s.repo.GetFeed(ctx, id)
			if err != nil {
				return domain.Feed{}, fmt.Errorf("get feed %d: %w", id, err)
			}
			// Renaming a feed to the name it already has is a no-op, not a conflict.
			if !strings.EqualFold(current.Title, *input.Title) {
				return domain.Feed{}, fmt.Errorf("update %q: %w", *input.Title, domain.ErrDuplicateTitle)
			}
		}
	}

	updated, err := s.repo.UpdateFeed(ctx, id, input)
	if err != nil {
		return domain.Feed{}, fmt.Errorf("update feed %d: %w", id, err)
	}
	return updated, nil
}
