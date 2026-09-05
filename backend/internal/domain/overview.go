package domain

import "time"

// SystemOverview is the read model behind the main page: one subscribed system as a
// single collapsed row. It carries no incidents on purpose — a card fetches those for
// itself when the user expands it.
//
// CurrentStatus lives here rather than on Feed because it is *derived* from the newest
// incident, not stored (the feed table has no such column). The HTTP adapter is what
// folds it into the wire representation of a feed.
type SystemOverview struct {
	Feed          Feed
	Indicator     Indicator
	CurrentStatus *Status    // nil when the system has no incidents
	LastUpdatedAt *time.Time // newest incident's timestamp; nil when there is none
}

// NewSystemOverview assembles the row for one feed from its newest incident, which is
// nil when the feed has none. The traffic-light follows three rules, in order:
//
//  1. there is an incident      → its status decides the colour;
//  2. no incident, but the feed has been polled successfully → operational, because we
//     looked and nothing was wrong;
//  3. otherwise → unknown: the feed was never polled successfully, so we genuinely have
//     no idea.
//
// A failed *latest* poll (Feed.LastError) deliberately does not repaint a system whose
// last known status was good: the error travels as feed data for the expanded card
// instead of masquerading as an incident.
func NewSystemOverview(f Feed, latest *StatusItem) SystemOverview {
	o := SystemOverview{Feed: f}

	if latest == nil {
		if f.LastSuccessAt != nil {
			o.Indicator = IndicatorOperational
		} else {
			o.Indicator = IndicatorUnknown
		}
		return o
	}

	status := latest.Status
	o.Indicator = status.Indicator()
	o.CurrentStatus = &status

	if t := latest.LatestTimestamp(); !t.IsZero() {
		o.LastUpdatedAt = &t
	}
	return o
}
