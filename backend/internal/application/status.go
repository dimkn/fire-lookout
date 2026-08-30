// Package application holds the use cases of the status-page reader. It orchestrates
// the domain ports and contains no transport or persistence detail of its own.
package application

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"fire-lookout/backend/internal/domain"
)

// StatusService serves the read side of the status page: the main-page overview and one
// system's incident history.
type StatusService struct {
	repo domain.StatusRepository
}

// NewStatusService wires the service to a repository.
func NewStatusService(repo domain.StatusRepository) *StatusService {
	return &StatusService{repo: repo}
}

// Overview returns one row per subscribed system, ordered by title (case-insensitively,
// ties broken by id) so the result is stable across calls. The ordering is a courtesy:
// the UI treats the response as an unordered set.
func (s *StatusService) Overview(ctx context.Context) ([]domain.SystemOverview, error) {
	feeds, err := s.repo.ListFeeds(ctx)
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}

	latest, err := s.repo.ListLatestItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("list latest items: %w", err)
	}

	latestByFeed := make(map[int64]domain.StatusItem, len(latest))
	for _, item := range latest {
		latestByFeed[item.FeedID] = item
	}

	overviews := make([]domain.SystemOverview, 0, len(feeds))
	for _, f := range feeds {
		var newest *domain.StatusItem
		if item, ok := latestByFeed[f.ID]; ok {
			newest = &item
		}
		overviews = append(overviews, domain.NewSystemOverview(f, newest))
	}

	slices.SortStableFunc(overviews, func(a, b domain.SystemOverview) int {
		if c := cmp.Compare(strings.ToLower(a.Feed.Title), strings.ToLower(b.Feed.Title)); c != 0 {
			return c
		}
		return cmp.Compare(a.Feed.ID, b.Feed.ID)
	})

	return overviews, nil
}

// FeedItems returns one system's status items, newest first. limit is clamped to the
// published bounds (0 or less means domain.DefaultItemLimit), and the feed's existence
// is checked first so an unknown id fails with domain.ErrNotFound instead of looking
// like a system that simply has no incidents.
func (s *StatusService) FeedItems(ctx context.Context, feedID int64, since *time.Time, limit int) ([]domain.StatusItem, error) {
	if _, err := s.repo.GetFeed(ctx, feedID); err != nil {
		return nil, fmt.Errorf("get feed %d: %w", feedID, err)
	}

	switch {
	case limit <= 0:
		limit = domain.DefaultItemLimit
	case limit > domain.MaxItemLimit:
		limit = domain.MaxItemLimit
	}

	items, err := s.repo.ListItems(ctx, feedID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("list items for feed %d: %w", feedID, err)
	}
	return items, nil
}
