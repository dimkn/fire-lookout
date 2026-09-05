// Package feed is the driven adapter that reads remote RSS/Atom endpoints, wrapping
// gofeed behind the domain.FeedFetcher port.
package feed

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"fire-lookout/backend/internal/domain"
)

// DefaultTimeout bounds a single fetch. A status page that cannot answer in ten seconds is
// no use to us, and an unbounded wait would hold the subscribe request open indefinitely.
const DefaultTimeout = 10 * time.Second

// Fetcher reads and parses feeds over HTTP.
type Fetcher struct {
	parser *gofeed.Parser
}

// NewFetcher builds a fetcher whose requests time out after the given duration; pass zero
// for DefaultTimeout.
func NewFetcher(timeout time.Duration) *Fetcher {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	parser := gofeed.NewParser()
	parser.Client = &http.Client{Timeout: timeout}
	return &Fetcher{parser: parser}
}

// Fetch retrieves and parses the feed at url.
//
// Every failure — DNS, connection refused, a timeout, a non-2xx status, or a body that is
// not a feed — comes back as a wrapped domain.ErrFeedUnreachable, because the caller only
// needs to know the endpoint is unusable. The underlying cause is wrapped too, so logs
// keep the detail.
func (f *Fetcher) Fetch(ctx context.Context, url string) (domain.FetchedFeed, error) {
	parsed, err := f.parser.ParseURLWithContext(url, ctx)
	if err != nil {
		return domain.FetchedFeed{}, fmt.Errorf("read feed %q: %w: %w", url, err, domain.ErrFeedUnreachable)
	}
	if parsed == nil {
		return domain.FetchedFeed{}, fmt.Errorf("read feed %q: empty document: %w", url, domain.ErrFeedUnreachable)
	}

	// gofeed is deliberately lenient and also understands JSON Feed, so an unrelated
	// document like `{"status":"ok"}` parses "successfully" into an empty feed. Both RSS
	// and Atom require a channel/feed title, so a document with neither a title nor a
	// single entry is not a feed we can use — reject it rather than let a user subscribe
	// to an endpoint that will never yield an incident.
	if parsed.Title == "" && len(parsed.Items) == 0 {
		return domain.FetchedFeed{}, fmt.Errorf(
			"read feed %q: no feed title or entries: %w", url, domain.ErrFeedUnreachable)
	}

	return domain.FetchedFeed{Title: parsed.Title}, nil
}
