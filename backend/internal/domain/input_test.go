package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSanitizeSubscribeInputAcceptsGoodInput(t *testing.T) {
	in := SubscribeInput{
		URL:   "  https://www.githubstatus.com/history.atom  ",
		Title: "  GitHub  ",
	}

	got, err := in.Sanitize()
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}

	if got.URL != "https://www.githubstatus.com/history.atom" {
		t.Errorf("URL = %q, want it trimmed", got.URL)
	}
	if got.Title != "GitHub" {
		t.Errorf("Title = %q, want it trimmed", got.Title)
	}
	if got.GroupID != UngroupedID {
		t.Errorf("GroupID = %d, want %d", got.GroupID, UngroupedID)
	}
	if got.RefreshInterval != DefaultRefreshInterval {
		t.Errorf("RefreshInterval = %v, want the default %v", got.RefreshInterval, DefaultRefreshInterval)
	}
	if got.Enabled == nil || !*got.Enabled {
		t.Errorf("Enabled = %v, want true by default", got.Enabled)
	}
}

func TestSanitizeSubscribeInputRejectsEmptyStrings(t *testing.T) {
	tests := []struct {
		name      string
		in        SubscribeInput
		wantField string
	}{
		{
			name:      "missing title",
			in:        SubscribeInput{URL: "https://a.test/feed.atom"},
			wantField: "title",
		},
		{
			// Whitespace is not a name: it must fail, not become an invisible title.
			name:      "whitespace-only title",
			in:        SubscribeInput{URL: "https://a.test/feed.atom", Title: "   \t  "},
			wantField: "title",
		},
		{
			name:      "missing url",
			in:        SubscribeInput{Title: "GitHub"},
			wantField: "url",
		},
		{
			name:      "whitespace-only url",
			in:        SubscribeInput{Title: "GitHub", URL: "  "},
			wantField: "url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.in.Sanitize()

			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want an ErrInvalidInput", err)
			}
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("error = %v, want a ValidationError", err)
			}
			if ve.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", ve.Field, tt.wantField)
			}
			if ve.Message == "" {
				t.Error("Message is empty, want something showable to the user")
			}
		})
	}
}

func TestSanitizeSubscribeInputRejectsUnusableURLs(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"no scheme", "www.githubstatus.com/history.atom"},
		{"not http", "ftp://a.test/feed.atom"},
		{"file scheme", "file:///etc/passwd"},
		{"scheme only", "https://"},
		{"nonsense", "::not a url::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SubscribeInput{Title: "GitHub", URL: tt.url}.Sanitize()

			var ve ValidationError
			if !errors.As(err, &ve) || ve.Field != "url" {
				t.Fatalf("error = %v, want a url ValidationError", err)
			}
		})
	}
}

func TestSanitizeSubscribeInputHonoursExplicitValues(t *testing.T) {
	disabled := false
	in := SubscribeInput{
		URL:             "https://a.test/feed.atom",
		Title:           "GitHub",
		GroupID:         3,
		RefreshInterval: 90 * time.Second,
		Enabled:         &disabled,
	}

	got, err := in.Sanitize()
	if err != nil {
		t.Fatalf("Sanitize() error = %v", err)
	}

	if got.GroupID != 3 {
		t.Errorf("GroupID = %d, want 3", got.GroupID)
	}
	if got.RefreshInterval != 90*time.Second {
		t.Errorf("RefreshInterval = %v, want 90s", got.RefreshInterval)
	}
	if got.Enabled == nil || *got.Enabled {
		t.Errorf("Enabled = %v, want false", got.Enabled)
	}
}

func TestSanitizeSubscribeInputRejectsTooFrequentPolling(t *testing.T) {
	in := SubscribeInput{
		URL:             "https://a.test/feed.atom",
		Title:           "GitHub",
		RefreshInterval: MinRefreshInterval - time.Second,
	}

	_, err := in.Sanitize()

	var ve ValidationError
	if !errors.As(err, &ve) || ve.Field != "refresh_interval_sec" {
		t.Fatalf("error = %v, want a refresh_interval_sec ValidationError", err)
	}
}
