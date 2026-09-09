package domain

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

func TestNewSystemOverviewDerivesIndicatorFromLatestItem(t *testing.T) {
	tests := []struct {
		name          string
		status        Status
		wantIndicator Indicator
	}{
		{"resolved is operational", StatusResolved, IndicatorOperational},
		{"monitoring is degraded", StatusMonitoring, IndicatorDegraded},
		{"maintenance is degraded", StatusMaintenance, IndicatorDegraded},
		{"investigating is an outage", StatusInvestigating, IndicatorOutage},
		{"identified is an outage", StatusIdentified, IndicatorOutage},
		{"unknown stays unknown", StatusUnknown, IndicatorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			published := ts("2026-08-17T10:00:00Z")
			feed := Feed{ID: 7, Title: "GitHub", Enabled: true, LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z"))}
			latest := &StatusItem{FeedID: 7, Status: tt.status, PublishedAt: published}

			got := NewSystemOverview(feed, latest)

			if got.Indicator != tt.wantIndicator {
				t.Errorf("Indicator = %q, want %q", got.Indicator, tt.wantIndicator)
			}
			if got.CurrentStatus == nil || *got.CurrentStatus != tt.status {
				t.Errorf("CurrentStatus = %v, want %q", got.CurrentStatus, tt.status)
			}
			if got.LastUpdatedAt == nil || !got.LastUpdatedAt.Equal(published) {
				t.Errorf("LastUpdatedAt = %v, want %v", got.LastUpdatedAt, published)
			}
			if got.Feed.ID != feed.ID || got.Feed.Title != feed.Title {
				t.Errorf("Feed = %+v, want it passed through unchanged", got.Feed)
			}
		})
	}
}

func TestNewSystemOverviewWithoutItems(t *testing.T) {
	tests := []struct {
		name          string
		feed          Feed
		wantIndicator Indicator
	}{
		{
			// Polled successfully and nothing came back: nothing is wrong.
			name:          "polled successfully is operational",
			feed:          Feed{ID: 1, Enabled: true, LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z"))},
			wantIndicator: IndicatorOperational,
		},
		{
			name:          "never polled is unknown",
			feed:          Feed{ID: 2, Enabled: true},
			wantIndicator: IndicatorUnknown,
		},
		{
			// Attempted, never succeeded: we genuinely do not know.
			name:          "attempted but never succeeded is unknown",
			feed:          Feed{ID: 3, Enabled: true, LastFetchedAt: ptr(ts("2026-08-18T09:00:00Z")), LastError: "dial tcp: timeout"},
			wantIndicator: IndicatorUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewSystemOverview(tt.feed, nil)

			if got.Indicator != tt.wantIndicator {
				t.Errorf("Indicator = %q, want %q", got.Indicator, tt.wantIndicator)
			}
			if got.CurrentStatus != nil {
				t.Errorf("CurrentStatus = %v, want nil", *got.CurrentStatus)
			}
			if got.LastUpdatedAt != nil {
				t.Errorf("LastUpdatedAt = %v, want nil", *got.LastUpdatedAt)
			}
		})
	}
}

func TestNewSystemOverviewLastErrorDoesNotOverrideTheLight(t *testing.T) {
	// A failing poll does not repaint a known-good system: the light reflects the
	// latest known status, and the error is surfaced separately as feed data.
	feed := Feed{
		ID:            9,
		Enabled:       true,
		LastFetchedAt: ptr(ts("2026-08-18T09:05:00Z")),
		LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z")),
		LastError:     "502 Bad Gateway",
	}
	latest := &StatusItem{FeedID: 9, Status: StatusResolved, PublishedAt: ts("2026-08-17T10:00:00Z")}

	got := NewSystemOverview(feed, latest)

	if got.Indicator != IndicatorOperational {
		t.Errorf("Indicator = %q, want %q", got.Indicator, IndicatorOperational)
	}
	if got.Feed.LastError != feed.LastError {
		t.Errorf("LastError = %q, want it preserved as %q", got.Feed.LastError, feed.LastError)
	}
}

func TestStatusItemLatestTimestamp(t *testing.T) {
	published := ts("2026-08-17T10:00:00Z")
	updated := ts("2026-08-17T12:30:00Z")
	fetched := ts("2026-08-18T09:00:00Z")

	tests := []struct {
		name string
		item StatusItem
		want time.Time
	}{
		{
			name: "prefers the newer of published and updated",
			item: StatusItem{PublishedAt: published, UpdatedAt: updated, FetchedAt: fetched},
			want: updated,
		},
		{
			name: "keeps published when it is the newer one",
			item: StatusItem{PublishedAt: updated, UpdatedAt: published, FetchedAt: fetched},
			want: updated,
		},
		{
			name: "falls back to fetched when the feed carried no timestamps",
			item: StatusItem{FetchedAt: fetched},
			want: fetched,
		},
		{
			name: "zero when there is nothing at all",
			item: StatusItem{},
			want: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.LatestTimestamp(); !got.Equal(tt.want) {
				t.Errorf("LatestTimestamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewSystemOverviewUsesItemLatestTimestamp(t *testing.T) {
	updated := ts("2026-08-17T12:30:00Z")
	latest := &StatusItem{
		Status:      StatusMonitoring,
		PublishedAt: ts("2026-08-17T10:00:00Z"),
		UpdatedAt:   updated,
	}

	got := NewSystemOverview(Feed{ID: 4, Enabled: true}, latest)

	if got.LastUpdatedAt == nil || !got.LastUpdatedAt.Equal(updated) {
		t.Errorf("LastUpdatedAt = %v, want %v", got.LastUpdatedAt, updated)
	}
}

func TestNewSystemOverviewNilTimestampWhenItemHasNone(t *testing.T) {
	got := NewSystemOverview(Feed{ID: 5, Enabled: true}, &StatusItem{Status: StatusResolved})

	if got.LastUpdatedAt != nil {
		t.Errorf("LastUpdatedAt = %v, want nil", *got.LastUpdatedAt)
	}
	if got.Indicator != IndicatorOperational {
		t.Errorf("Indicator = %q, want %q", got.Indicator, IndicatorOperational)
	}
}

// Rule 0: a paused system reports "unknown" however good its last known status was. We are
// not watching it, so keeping the old colour would be a claim we can no longer support.
func TestNewSystemOverviewReportsUnknownWhilePaused(t *testing.T) {
	tests := []struct {
		name   string
		latest *StatusItem
	}{
		{
			name:   "with a healthy last incident",
			latest: &StatusItem{Status: StatusResolved, PublishedAt: ts("2026-08-18T08:00:00Z")},
		},
		{
			name:   "with an open incident",
			latest: &StatusItem{Status: StatusInvestigating, PublishedAt: ts("2026-08-18T08:00:00Z")},
		},
		{
			name:   "with no incidents at all",
			latest: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := Feed{ID: 1, Title: "Paused", Enabled: false, LastSuccessAt: ptr(ts("2026-08-18T09:00:00Z"))}

			got := NewSystemOverview(feed, tt.latest)

			if got.Indicator != IndicatorUnknown {
				t.Errorf("Indicator = %q, want unknown while paused", got.Indicator)
			}
			if got.CurrentStatus == nil || *got.CurrentStatus != StatusUnknown {
				t.Errorf("CurrentStatus = %v, want an explicit unknown", got.CurrentStatus)
			}
		})
	}
}

// A paused system reports no timestamp either: "last changed at 09:00" sitting beside a grey
// "we don't know" reads as though the pair were current. The row stays bare instead.
func TestNewSystemOverviewHidesTheTimestampWhilePaused(t *testing.T) {
	feed := Feed{ID: 1, Enabled: false}
	latest := &StatusItem{Status: StatusResolved, PublishedAt: ts("2026-08-18T08:00:00Z")}

	got := NewSystemOverview(feed, latest)

	if got.LastUpdatedAt != nil {
		t.Errorf("LastUpdatedAt = %v, want nil while paused", got.LastUpdatedAt)
	}
}

// Hidden, not lost: the stored entry is untouched, so resuming reports it again.
func TestNewSystemOverviewRestoresTheTimestampWhenReEnabled(t *testing.T) {
	published := ts("2026-08-18T08:00:00Z")
	latest := &StatusItem{Status: StatusResolved, PublishedAt: published}

	paused := NewSystemOverview(Feed{ID: 1, Enabled: false}, latest)
	resumed := NewSystemOverview(Feed{ID: 1, Enabled: true}, latest)

	if paused.LastUpdatedAt != nil {
		t.Errorf("paused LastUpdatedAt = %v, want nil", paused.LastUpdatedAt)
	}
	if resumed.LastUpdatedAt == nil || !resumed.LastUpdatedAt.Equal(published) {
		t.Errorf("resumed LastUpdatedAt = %v, want %v back", resumed.LastUpdatedAt, published)
	}
}

// Nothing is destroyed by pausing: re-enabling brings the stored status straight back.
func TestNewSystemOverviewRestoresTheStatusWhenReEnabled(t *testing.T) {
	latest := &StatusItem{Status: StatusInvestigating, PublishedAt: ts("2026-08-18T08:00:00Z")}

	paused := NewSystemOverview(Feed{ID: 1, Enabled: false}, latest)
	resumed := NewSystemOverview(Feed{ID: 1, Enabled: true}, latest)

	if paused.Indicator != IndicatorUnknown {
		t.Errorf("paused indicator = %q, want unknown", paused.Indicator)
	}
	if resumed.Indicator != IndicatorOutage {
		t.Errorf("resumed indicator = %q, want the real status back", resumed.Indicator)
	}
}
