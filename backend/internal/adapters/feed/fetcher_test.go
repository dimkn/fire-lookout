package feed

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fire-lookout/backend/internal/domain"
)

const atomFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>GitHub Status - Incident History</title>
  <updated>2026-08-18T09:05:00Z</updated>
  <entry>
    <id>tag:githubstatus.com,2026:Incident/1</id>
    <title>Elevated error rates</title>
    <updated>2026-08-18T09:02:00Z</updated>
  </entry>
</feed>`

const rssFeed = `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Datadog Status</title>
  <item><title>Delayed ingestion</title><guid>dd-1</guid></item>
</channel></rss>`

func serve(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func TestFetchReadsAnAtomFeed(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(atomFeed))
	})

	got, err := NewFetcher(0).Fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.Title != "GitHub Status - Incident History" {
		t.Errorf("Title = %q, want the feed's own title", got.Title)
	}
}

func TestFetchReadsAnRSSFeed(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssFeed))
	})

	got, err := NewFetcher(0).Fetch(context.Background(), url)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.Title != "Datadog Status" {
		t.Errorf("Title = %q, want %q", got.Title, "Datadog Status")
	}
}

func TestFetchRejectsWhatIsNotAFeed(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "an html page",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte("<!doctype html><html><body>Not a feed</body></html>"))
			},
		},
		{
			name: "json",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			},
		},
		{
			name: "an empty body",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "truncated xml",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`<?xml version="1.0"?><feed><title>Half a`))
			},
		},
		{
			name: "a server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
		{
			name: "a not-found page",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := serve(t, tt.handler)

			_, err := NewFetcher(0).Fetch(context.Background(), url)

			if !errors.Is(err, domain.ErrFeedUnreachable) {
				t.Errorf("error = %v, want ErrFeedUnreachable", err)
			}
		})
	}
}

func TestFetchRejectsAnUnreachableHost(t *testing.T) {
	// Start a server just to get a free port, then close it: nothing is listening.
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := NewFetcher(0).Fetch(context.Background(), url)

	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Errorf("error = %v, want ErrFeedUnreachable", err)
	}
}

func TestFetchGivesUpOnASlowServer(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		_, _ = w.Write([]byte(atomFeed))
	})

	_, err := NewFetcher(50*time.Millisecond).Fetch(context.Background(), url)

	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Errorf("error = %v, want ErrFeedUnreachable after the timeout", err)
	}
}

func TestFetchStopsWhenTheCallerCancels(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		_, _ = w.Write([]byte(atomFeed))
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := NewFetcher(0).Fetch(ctx, url)

	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Errorf("error = %v, want ErrFeedUnreachable when the context is cancelled", err)
	}
}

// The port promises a domain error; it must not leak a transport error type either.
func TestFetchAlwaysReportsTheDomainSentinel(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	_, err := NewFetcher(0).Fetch(context.Background(), url)

	if !errors.Is(err, domain.ErrFeedUnreachable) {
		t.Fatalf("error = %v, want ErrFeedUnreachable", err)
	}
	if err.Error() == "" {
		t.Error("error message is empty, want the underlying cause preserved for logs")
	}
}
