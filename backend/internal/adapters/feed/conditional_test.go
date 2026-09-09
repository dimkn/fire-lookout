package feed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

const statuspageFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Acme Status - Incident History</title>
  <updated>2026-08-24T10:00:00Z</updated>
  <entry>
    <id>tag:acme,2026:Incident/42</id>
    <title>Elevated error rates</title>
    <link href="https://status.acme.test/incidents/42"/>
    <published>2026-08-24T09:00:00Z</published>
    <updated>2026-08-24T09:30:00Z</updated>
    <content type="html">&lt;p&gt;&lt;strong&gt;Investigating&lt;/strong&gt; - We are looking into errors.&lt;/p&gt;</content>
  </entry>
</feed>`

func TestFetchSendsTheValidatorsItWasGiven(t *testing.T) {
	var got http.Header
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(statuspageFeed))
	})

	_, err := NewFetcher(0, "1.2.3").Fetch(context.Background(), domain.FetchRequest{
		URL:          url,
		ETag:         `W/"abc"`,
		LastModified: "Mon, 24 Aug 2026 10:00:00 GMT",
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got.Get("If-None-Match") != `W/"abc"` {
		t.Errorf("If-None-Match = %q, want the stored etag", got.Get("If-None-Match"))
	}
	if got.Get("If-Modified-Since") != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Errorf("If-Modified-Since = %q, want the stored date", got.Get("If-Modified-Since"))
	}
	if ua := got.Get("User-Agent"); !strings.Contains(ua, "fire-lookout/1.2.3") {
		t.Errorf("User-Agent = %q, want it to identify the tool and version", ua)
	}
	if !strings.Contains(got.Get("Accept"), "atom") {
		t.Errorf("Accept = %q, want feed types listed", got.Get("Accept"))
	}
}

func TestFetchOmitsValidatorsItDoesNotHave(t *testing.T) {
	var got http.Header
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(statuspageFeed))
	})

	if _, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url}); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if _, present := got["If-None-Match"]; present {
		t.Error("If-None-Match was sent without a stored etag")
	}
	if _, present := got["If-Modified-Since"]; present {
		t.Error("If-Modified-Since was sent without a stored date")
	}
}

func TestFetchReportsNotModified(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `W/"abc"`)
		w.WriteHeader(http.StatusNotModified)
	})

	got, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{
		URL: url, ETag: `W/"abc"`,
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v, want a 304 to be a success", err)
	}

	if !got.NotModified {
		t.Error("NotModified = false, want true")
	}
	if len(got.Items) != 0 {
		t.Errorf("got %d items, want none from a 304", len(got.Items))
	}
	if got.ETag != `W/"abc"` {
		t.Errorf("ETag = %q, want the validator echoed back", got.ETag)
	}
}

// The real round trip: first poll stores the validator, second poll gets a 304 back.
func TestFetchRoundTripsAnETag(t *testing.T) {
	const etag = `"v1"`
	var requests int
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("If-None-Match") == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", "Mon, 24 Aug 2026 10:00:00 GMT")
		_, _ = w.Write([]byte(statuspageFeed))
	})
	fetcher := NewFetcher(0, "test")

	first, err := fetcher.Fetch(context.Background(), domain.FetchRequest{URL: url})
	if err != nil {
		t.Fatalf("first Fetch() error = %v", err)
	}
	if first.NotModified {
		t.Fatal("the first fetch reported not-modified, want the body")
	}
	if first.ETag != etag || first.LastModified != "Mon, 24 Aug 2026 10:00:00 GMT" {
		t.Fatalf("validators = %q / %q, want them captured", first.ETag, first.LastModified)
	}

	second, err := fetcher.Fetch(context.Background(), domain.FetchRequest{
		URL: url, ETag: first.ETag, LastModified: first.LastModified,
	})
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if !second.NotModified {
		t.Error("the second fetch re-read the body, want a 304")
	}
	if requests != 2 {
		t.Errorf("made %d requests, want 2", requests)
	}
}

// 429 is its own thing: the provider is throttling us, which says nothing about the feed.
func TestFetchReportsRateLimitingSeparately(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})

	if !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
	if errors.Is(err, domain.ErrFeedUnreachable) {
		t.Error("a 429 must not also read as an unreachable feed")
	}
}

func TestFetchMapsEntries(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(statuspageFeed))
	})

	got, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}

	item := got.Items[0]
	if item.GUID != "tag:acme,2026:Incident/42" {
		t.Errorf("GUID = %q, want the atom id", item.GUID)
	}
	if item.Title != "Elevated error rates" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.Link != "https://status.acme.test/incidents/42" {
		t.Errorf("Link = %q", item.Link)
	}
	if !item.PublishedAt.Equal(time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("PublishedAt = %v", item.PublishedAt)
	}
	if !item.UpdatedAt.Equal(time.Date(2026, 8, 24, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("UpdatedAt = %v", item.UpdatedAt)
	}
	if !strings.Contains(item.ContentHTML, "<strong>") {
		t.Errorf("ContentHTML = %q, want the markup preserved", item.ContentHTML)
	}
	if item.ContentText != "Investigating - We are looking into errors." {
		t.Errorf("ContentText = %q, want tags stripped and entities decoded", item.ContentText)
	}
	// Deriving status is the application's job, not the adapter's.
	if item.Status != "" {
		t.Errorf("Status = %q, want it left for the application to derive", item.Status)
	}
}

func TestFetchInventsAGUIDWhenTheFeedOmitsOne(t *testing.T) {
	const noGUID = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Bare</title>
  <item><title>Something happened</title><pubDate>Mon, 24 Aug 2026 09:00:00 GMT</pubDate></item>
</channel></rss>`

	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(noGUID))
	})

	got, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	// The column is NOT NULL and the key must be stable across polls.
	if got.Items[0].GUID == "" {
		t.Error("GUID is empty, want a derived key")
	}

	again, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})
	if err != nil {
		t.Fatalf("second Fetch() error = %v", err)
	}
	if again.Items[0].GUID != got.Items[0].GUID {
		t.Errorf("derived GUID changed between polls: %q then %q", got.Items[0].GUID, again.Items[0].GUID)
	}
}

// A provider with nothing wrong publishes a feed with no entries; that is not an error.
func TestFetchAcceptsATitledFeedWithNoEntries(t *testing.T) {
	const empty = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>All Quiet Status</title>
  <updated>2026-08-24T10:00:00Z</updated>
</feed>`

	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(empty))
	})

	got, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})
	if err != nil {
		t.Fatalf("Fetch() error = %v, want an empty-but-valid feed to be fine", err)
	}
	if got.Title != "All Quiet Status" {
		t.Errorf("Title = %q", got.Title)
	}
	if len(got.Items) != 0 {
		t.Errorf("got %d items, want none", len(got.Items))
	}
}

func TestFetchStopsReadingAnEnormousBody(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		// A well-formed prologue followed by far more padding than the cap allows.
		_, _ = fmt.Fprint(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Huge</title><entry><title>`)
		chunk := strings.Repeat("x", 1<<16)
		for written := 0; written < (maxBodyBytes + (1 << 20)); written += len(chunk) {
			if _, err := fmt.Fprint(w, chunk); err != nil {
				return
			}
		}
	})

	_, err := NewFetcher(0, "test").Fetch(context.Background(), domain.FetchRequest{URL: url})

	// Truncated at the cap, so it cannot parse — the point is that it returns rather than
	// reading for ever.
	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Errorf("error = %v, want the capped read to fail as unreachable", err)
	}
}

func TestStripTags(t *testing.T) {
	tests := []struct {
		name   string
		markup string
		want   string
	}{
		{"plain text is untouched", "Resolved - all clear.", "Resolved - all clear."},
		{"tags go", "<p>Resolved - all clear.</p>", "Resolved - all clear."},
		{"entities are decoded", "&lt;not a tag&gt; &amp; more", "<not a tag> & more"},
		{
			name:   "adjacent blocks keep a word boundary",
			markup: "<p>Resolved</p><p>All clear.</p>",
			want:   "Resolved All clear.",
		},
		{
			name:   "attributes and links",
			markup: `<a href="https://x.test">Status page</a>`,
			want:   "Status page",
		},
		{"newlines collapse", "line one\n\n   line two", "line one line two"},
		{"empty stays empty", "", ""},
		{"markup only", "<br/><hr/>", ""},
		{
			name:   "a script body is text, not markup",
			markup: "<script>alert(1)</script>Real text",
			want:   "alert(1) Real text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripTags(tt.markup); got != tt.want {
				t.Errorf("stripTags(%q) = %q, want %q", tt.markup, got, tt.want)
			}
		})
	}
}

func TestFetchDoesNotBlowUpOnANilEntry(t *testing.T) {
	// gofeed never hands us a nil item, but the mapper guards anyway; this pins the guard.
	if items := toStatusItems(nil); len(items) != 0 {
		t.Errorf("toStatusItems(nil) = %v, want empty", items)
	}
}
