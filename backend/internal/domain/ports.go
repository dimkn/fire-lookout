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

	// FeedExistsByURL reports whether a feed with this exact url is already subscribed.
	FeedExistsByURL(ctx context.Context, url string) (bool, error)

	// FeedExistsByTitle reports whether a feed already uses this name, compared
	// case-insensitively.
	FeedExistsByTitle(ctx context.Context, title string) (bool, error)

	// CreateFeed stores a new feed and returns it with its id and timestamps filled in.
	// It returns a wrapped ErrDuplicateURL or ErrDuplicateTitle when the row collides
	// with an existing one.
	CreateFeed(ctx context.Context, feed Feed) (Feed, error)
}

// FetchedFeed is what a remote RSS/Atom endpoint yielded. Only the metadata the current
// use cases need is carried; the poller will extend it with parsed entries.
type FetchedFeed struct {
	Title string // the feed's own <title>, empty when it declares none
}

// FeedFetcher is the driven port for reading a remote RSS/Atom endpoint. The subscribe
// flow uses it to prove a URL really is a feed before anything is stored.
//
// Implementations return a wrapped ErrFeedUnreachable when the endpoint cannot be read or
// does not parse as a feed.
type FeedFetcher interface {
	Fetch(ctx context.Context, url string) (FetchedFeed, error)
}
