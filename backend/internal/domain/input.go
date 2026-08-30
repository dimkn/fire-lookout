package domain

import "time"

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
