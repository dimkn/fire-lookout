package domain

import (
	"regexp"
	"strings"
)

// marker is one recognised update prefix and the state it implies.
type marker struct {
	word   string
	status Status
}

// The Atlassian Statuspage vocabulary. "Completed" maps to resolved because a finished
// maintenance window means nothing is wrong any more, and "Update" maps to investigating
// because a provider only posts one while an incident is still open.
var markers = []marker{
	{"resolved", StatusResolved},
	{"completed", StatusResolved},
	{"monitoring", StatusMonitoring},
	{"identified", StatusIdentified},
	{"investigating", StatusInvestigating},
	{"update", StatusInvestigating},
	{"in progress", StatusMaintenance},
	{"scheduled", StatusMaintenance},
	{"maintenance", StatusMaintenance},
}

// markerRE matches a marker in its update-prefix form — "Resolved - ", "Monitoring: " —
// which is how Statuspage introduces each update. proseRE matches the same words anywhere,
// used only as a fallback so a title like "Scheduled maintenance: …" is still understood.
var (
	markerRE = regexp.MustCompile(`(?i)\b(` + markerAlternation() + `)\b\s*[-–—:]`)
	proseRE  = regexp.MustCompile(`(?i)\b(` + markerAlternation() + `)\b`)
)

func markerAlternation() string {
	words := make([]string, 0, len(markers))
	for _, m := range markers {
		words = append(words, regexp.QuoteMeta(m.word))
	}
	return strings.Join(words, "|")
}

// ParseStatus derives an incident's current state from a feed entry, best-effort.
//
// Statuspage-style feeds concatenate an incident's updates newest-first, so the FIRST
// marker in the body is the current state and everything after it is history. That ordering
// is a convention rather than a guarantee — it is the assumption this function rests on.
//
// The body is preferred over the title, the update-prefix form ("Resolved - …") is preferred
// over the same word appearing in prose, and anything unrecognised — Azure, incident.io, a
// plain announcement — resolves to StatusUnknown rather than to a guess.
func ParseStatus(title, body string) Status {
	for _, text := range []string{body, title} {
		if status, ok := firstMarker(text); ok {
			return status
		}
	}
	return StatusUnknown
}

// firstMarker returns the status of the earliest marker in text, trying the update-prefix
// form before falling back to a bare word.
func firstMarker(text string) (Status, bool) {
	if text == "" {
		return StatusUnknown, false
	}
	for _, re := range []*regexp.Regexp{markerRE, proseRE} {
		if match := re.FindStringSubmatch(text); match != nil {
			return statusForWord(match[1]), true
		}
	}
	return StatusUnknown, false
}

func statusForWord(word string) Status {
	word = strings.ToLower(strings.TrimSpace(word))
	for _, m := range markers {
		if m.word == word {
			return m.status
		}
	}
	return StatusUnknown
}
