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

	// DueFeeds returns the enabled feeds whose cadence has elapsed by now, plus any that
	// have never been polled. Disabled feeds are never due.
	DueFeeds(ctx context.Context, now time.Time) ([]Feed, error)

	// SaveItems upserts a poll's entries, keyed on (feed_id, guid), so an incident that
	// gains an update is revised in place rather than duplicated.
	SaveItems(ctx context.Context, feedID int64, items []StatusItem) error

	// RecordPoll writes the bookkeeping for one poll attempt.
	RecordPoll(ctx context.Context, result PollResult) error

	// PruneItems deletes a feed's items older than before, and reports how many went.
	PruneItems(ctx context.Context, feedID int64, before time.Time) (int64, error)
}

// PollOutcome is what one poll attempt amounted to.
type PollOutcome string

const (
	// PollUpdated means the feed was read and its entries stored.
	PollUpdated PollOutcome = "updated"
	// PollNotModified means the provider answered 304 — a successful poll that told us
	// nothing changed.
	PollNotModified PollOutcome = "not_modified"
	// PollRateLimited means the provider asked us to back off. Not a failure: the attempt
	// is recorded so the feed waits a full interval, and nothing is marked broken.
	PollRateLimited PollOutcome = "rate_limited"
	// PollFailed means the endpoint could not be read.
	PollFailed PollOutcome = "failed"
)

// PollResult is the bookkeeping for one poll attempt. Which columns it touches depends on
// Outcome — see the storage adapter.
type PollResult struct {
	FeedID       int64
	At           time.Time
	Outcome      PollOutcome
	ETag         string // validators to keep for next time (Updated / NotModified)
	LastModified string
	Error        string // only for PollFailed
}

// FetchRequest asks for one feed, carrying the validators from the last successful poll so
// an unchanged feed can answer 304.
type FetchRequest struct {
	URL          string
	ETag         string // sent as If-None-Match when set
	LastModified string // sent as If-Modified-Since when set
}

// FetchedFeed is what a remote RSS/Atom endpoint yielded.
type FetchedFeed struct {
	Title string // the feed's own <title>, empty when it declares none

	// Items are the parsed entries with Status left unset: deriving it is a domain rule the
	// application applies, not something the adapter decides.
	Items []StatusItem

	// NotModified reports a 304. Items and Title are empty in that case.
	NotModified bool

	// Validators to store for the next request, verbatim.
	ETag         string
	LastModified string
}

// FeedFetcher is the driven port for reading a remote RSS/Atom endpoint. The subscribe flow
// uses it to prove a URL really is a feed; the poller uses it to ingest entries.
//
// Implementations return a wrapped ErrFeedUnreachable when the endpoint cannot be read or
// does not parse as a feed, and a wrapped ErrRateLimited on HTTP 429.
type FeedFetcher interface {
	Fetch(ctx context.Context, req FetchRequest) (FetchedFeed, error)
}
