package domain

import (
	"context"
	"time"
)

// StatusRepository is the driven port for persisted feeds and their status items. It
// holds only the reads the current use cases need; writes arrive with the poller and
// the subscribe flow.
//
// Implementations return wrapped ErrNotFound when a feed does not exist.
type StatusRepository interface {
	// ListFeeds returns every subscribed feed.
	ListFeeds(ctx context.Context) ([]Feed, error)

	// ListLatestItems returns the newest status item of every feed that has one — at
	// most one item per feed, in no particular order. It exists so the overview can be
	// assembled without a query per feed.
	ListLatestItems(ctx context.Context) ([]StatusItem, error)

	// GetFeed returns one feed, or a wrapped ErrNotFound.
	GetFeed(ctx context.Context, id int64) (Feed, error)

	// ListItems returns a feed's status items, newest first. A non-nil since restricts
	// the result to items at or after that instant; limit is a hard maximum and is
	// expected to already be clamped by the caller.
	ListItems(ctx context.Context, feedID int64, since *time.Time, limit int) ([]StatusItem, error)
}
