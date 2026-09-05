package domain

import (
	"net/url"
	"strings"
	"time"
)

// MinRefreshInterval is the fastest poll cadence a feed may be given, mirroring the bound
// published in api/openapi.yaml: any positive number of seconds is valid.
//
// The scheduler's tick is the practical floor — it only looks for due feeds every
// scheduler.DefaultTick — so a cadence below that is effectively rounded up to it. That is a
// deployment characteristic rather than a rule about the input, which is why it is not
// enforced here.
const MinRefreshInterval = 1 * time.Second

// SubscribeInput is what a user provides to subscribe to a new feed. Only URL is
// required; empty/zero fields fall back to defaults or the feed's own metadata. The
// application validates the URL by fetching and parsing it once before persisting.
type SubscribeInput struct {
	URL             string
	Title           string        // optional; defaults to the feed's <title>
	GroupID         int64         // optional; 0 == Ungrouped
	RefreshInterval time.Duration // optional; 0 => DefaultRefreshInterval
	Enabled         *bool         // optional; nil => true
}

// Sanitize trims every string field, fills in the defaults for omitted values, and reports
// a ValidationError when what remains cannot be accepted. Trimming happens before the
// emptiness checks on purpose: whitespace is not a name.
//
// Messages are written for the person who typed the value — the HTTP adapter shows them
// verbatim.
func (in SubscribeInput) Sanitize() (SubscribeInput, error) {
	out := in
	out.Title = strings.TrimSpace(in.Title)
	out.URL = strings.TrimSpace(in.URL)

	if out.Title == "" {
		return SubscribeInput{}, ValidationError{Field: "title", Message: "Name must not be empty."}
	}
	if out.URL == "" {
		return SubscribeInput{}, ValidationError{Field: "url", Message: "RSS link must not be empty."}
	}
	if err := validateFeedURL(out.URL); err != nil {
		return SubscribeInput{}, err
	}

	switch {
	case out.RefreshInterval == 0:
		out.RefreshInterval = DefaultRefreshInterval
	case out.RefreshInterval < MinRefreshInterval:
		return SubscribeInput{}, ValidationError{
			Field:   "refresh_interval_sec",
			Message: "Refresh interval must be a positive number of seconds.",
		}
	}

	if out.Enabled == nil {
		enabled := true
		out.Enabled = &enabled
	}
	return out, nil
}

// validateFeedURL insists on an absolute http(s) URL with a host. Rejecting other schemes
// keeps the fetcher from being pointed at the local filesystem or anything else exotic.
func validateFeedURL(raw string) error {
	invalid := ValidationError{
		Field:   "url",
		Message: "RSS link must be a valid http(s) URL.",
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return invalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return invalid
	}
	if parsed.Host == "" {
		return invalid
	}
	return nil
}

// UpdateFeedInput is a partial update of a feed. Every field is a pointer so that a nil
// value means "leave unchanged", distinct from an explicit zero value.
type UpdateFeedInput struct {
	Title           *string
	GroupID         *int64
	RefreshInterval *time.Duration
	Enabled         *bool
}

// CreateGroupInput is the data needed to create a new group.
type CreateGroupInput struct {
	Name     string
	Position int
}

// UpdateGroupInput is a partial update of a group; nil fields are left unchanged.
type UpdateGroupInput struct {
	Name     *string
	Position *int
}
