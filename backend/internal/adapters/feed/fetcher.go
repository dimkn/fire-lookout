// Package feed is the driven adapter that reads remote RSS/Atom endpoints, wrapping gofeed
// behind the domain.FeedFetcher port.
//
// The HTTP request is ours rather than gofeed's: gofeed's ParseURL hides the response, and
// conditional GET needs both the request headers going out and the status and validators
// coming back.
package feed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"fire-lookout/backend/internal/domain"
)

const (
	// DefaultTimeout bounds a single fetch. A status page that cannot answer in ten seconds
	// is no use to us, and an unbounded wait would hold a poll or a subscribe request open.
	DefaultTimeout = 10 * time.Second

	// maxBodyBytes caps what we are willing to read from a third party.
	maxBodyBytes = 5 << 20 // 5 MiB

	acceptHeader = "application/atom+xml, application/rss+xml, application/xml;q=0.9, text/xml;q=0.9, */*;q=0.8"
)

// Fetcher reads and parses feeds over HTTP.
type Fetcher struct {
	client    *http.Client
	parser    *gofeed.Parser
	userAgent string
}

// NewFetcher builds a fetcher whose requests time out after the given duration (zero means
// DefaultTimeout) and identify themselves with the given version. Providers do block
// unidentified agents, so the User-Agent is not decoration.
func NewFetcher(timeout time.Duration, version string) *Fetcher {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if version == "" {
		version = "dev"
	}

	return &Fetcher{
		client:    &http.Client{Timeout: timeout},
		parser:    gofeed.NewParser(),
		userAgent: fmt.Sprintf("fire-lookout/%s (local status page)", version),
	}
}

// Fetch retrieves and parses the feed named by req.
//
// When req carries validators from a previous poll they are sent as If-None-Match /
// If-Modified-Since, and an unchanged feed answers 304 — which comes back as
// FetchedFeed{NotModified: true} rather than as an error.
//
// HTTP 429 becomes a wrapped domain.ErrRateLimited: the provider is throttling us, which
// says nothing about the feed. Everything else that goes wrong — DNS, refused connection,
// timeout, any other non-2xx, a body that is not a feed — becomes a wrapped
// domain.ErrFeedUnreachable, with the cause preserved for the logs.
func (f *Fetcher) Fetch(ctx context.Context, req domain.FetchRequest) (domain.FetchedFeed, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return domain.FetchedFeed{}, fmt.Errorf("build request for %q: %w: %w", req.URL, err, domain.ErrFeedUnreachable)
	}

	httpReq.Header.Set("User-Agent", f.userAgent)
	httpReq.Header.Set("Accept", acceptHeader)
	if req.ETag != "" {
		httpReq.Header.Set("If-None-Match", req.ETag)
	}
	if req.LastModified != "" {
		httpReq.Header.Set("If-Modified-Since", req.LastModified)
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		// No URL prefix here: net/http's error already opens with `Get "<url>":`, and this
		// message ends up on the card as "Last check failed: …".
		return domain.FetchedFeed{}, fmt.Errorf("%w: %w", err, domain.ErrFeedUnreachable)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode == http.StatusNotModified:
		// Nothing to parse. Echo back whatever validators the 304 carried, if any.
		return domain.FetchedFeed{
			NotModified:  true,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		}, nil

	case resp.StatusCode == http.StatusTooManyRequests:
		return domain.FetchedFeed{}, fmt.Errorf("get %q: %s: %w", req.URL, resp.Status, domain.ErrRateLimited)

	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return domain.FetchedFeed{}, fmt.Errorf("get %q: %s: %w", req.URL, resp.Status, domain.ErrFeedUnreachable)
	}

	parsed, err := f.parser.Parse(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return domain.FetchedFeed{}, fmt.Errorf("parse %q: %w: %w", req.URL, err, domain.ErrFeedUnreachable)
	}
	if parsed == nil {
		return domain.FetchedFeed{}, fmt.Errorf("parse %q: empty document: %w", req.URL, domain.ErrFeedUnreachable)
	}

	// gofeed is deliberately lenient and also understands JSON Feed, so an unrelated
	// document like `{"status":"ok"}` parses "successfully" into an empty feed. Both RSS and
	// Atom require a channel/feed title, so a document with neither a title nor a single
	// entry is not a feed we can use. A *titled* feed with no entries is perfectly normal —
	// a provider with no incidents — and stays valid.
	if parsed.Title == "" && len(parsed.Items) == 0 {
		return domain.FetchedFeed{}, fmt.Errorf(
			"parse %q: no feed title or entries: %w", req.URL, domain.ErrFeedUnreachable)
	}

	return domain.FetchedFeed{
		Title:        parsed.Title,
		Items:        toStatusItems(parsed.Items),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}, nil
}

// toStatusItems translates parsed entries. Status is left unset on purpose: deriving it is a
// domain rule the application applies.
func toStatusItems(entries []*gofeed.Item) []domain.StatusItem {
	items := make([]domain.StatusItem, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		html := entry.Content
		if html == "" {
			html = entry.Description
		}

		item := domain.StatusItem{
			GUID:        entryGUID(entry),
			Title:       entry.Title,
			Link:        entry.Link,
			ContentHTML: html,
			ContentText: stripTags(html),
		}
		if entry.PublishedParsed != nil {
			item.PublishedAt = entry.PublishedParsed.UTC()
		}
		if entry.UpdatedParsed != nil {
			item.UpdatedAt = entry.UpdatedParsed.UTC()
		}
		items = append(items, item)
	}
	return items
}

// entryGUID picks a stable key. Atom requires <id> and RSS usually carries <guid>, but the
// column is NOT NULL and feeds in the wild omit both — so fall back to the link, and finally
// to a digest of what the entry does say.
func entryGUID(entry *gofeed.Item) string {
	if entry.GUID != "" {
		return entry.GUID
	}
	if entry.Link != "" {
		return entry.Link
	}

	sum := sha256.Sum256([]byte(entry.Title + "|" + entry.Published + "|" + entry.Updated))
	return "sha256:" + hex.EncodeToString(sum[:16])
}
