package domain

import "time"

// UngroupedID is the reserved feed_group row that every feed defaults to. Deleting a
// group reparents its feeds here rather than orphaning them (see the storage schema:
// group_id NOT NULL DEFAULT 0 ... ON DELETE SET DEFAULT).
const UngroupedID int64 = 0

// Policy defaults for the poller and retention (see AGENTS.md §4 "Data model").
const (
	// DefaultRefreshInterval is the poll cadence used when a feed sets none.
	DefaultRefreshInterval = 5 * time.Minute
	// RetentionWindow is how long status items are kept before pruning ("last week or so").
	RetentionWindow = 7 * 24 * time.Hour
)

// Group is an arbitrary, user-defined bucket of feeds ("Dev tools", "Analytics", ...).
// The reserved Group with ID == UngroupedID always exists and cannot be deleted.
type Group struct {
	ID       int64
	Name     string
	Position int // display order
}

// Feed is a subscription to one RSS/Atom status feed.
type Feed struct {
	ID              int64
	URL             string        // the RSS/Atom endpoint
	Title           string        // user override, or the feed's own <title>
	GroupID         int64         // 0 == Ungrouped
	Enabled         bool          // false pauses polling without unsubscribing
	RefreshInterval time.Duration // poll cadence

	// Health bookkeeping (our clock), maintained by the poller. Together they answer:
	// did we try (LastFetchedAt), did it work (LastSuccessAt), what broke (LastError).
	LastFetchedAt *time.Time // last poll attempt, success or failure; nil until first poll
	LastSuccessAt *time.Time // last successful poll; nil until first success
	LastError     string     // last failure message; empty when healthy

	CreatedAt time.Time
	UpdatedAt time.Time
}

// StatusItem is a single incident / status entry parsed from a feed (one Atom <entry>
// or RSS <item>). ContentHTML is preserved verbatim as the source of truth; Status is a
// best-effort derivation from it and may be StatusUnknown.
type StatusItem struct {
	ID          int64
	FeedID      int64
	GUID        string // stable external id: Atom <id> / RSS <guid> / fallback to Link
	Title       string
	Link        string
	PublishedAt time.Time
	UpdatedAt   time.Time
	ContentHTML string
	ContentText string // stripped plaintext, for preview/search
	Status      Status
	FetchedAt   time.Time // when we last stored/updated this item
}
