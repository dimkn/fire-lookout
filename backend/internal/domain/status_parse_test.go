package domain

import "testing"

func TestParseStatusReadsTheStatuspageConvention(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  Status
	}{
		{
			name:  "investigating",
			title: "Elevated error rates for Actions and Pages",
			body:  "Investigating - We are looking into elevated error rates.",
			want:  StatusInvestigating,
		},
		{
			name:  "identified",
			title: "Delayed webhooks",
			body:  "Identified - The issue has been identified and a fix is being applied.",
			want:  StatusIdentified,
		},
		{
			name:  "monitoring",
			title: "Delayed log ingestion in US5",
			body:  "Monitoring - A fix has been applied and we are monitoring the backlog drain.",
			want:  StatusMonitoring,
		},
		{
			name:  "resolved",
			title: "Increased 522 errors in AMS",
			body:  "Resolved - Traffic through Amsterdam has been fully restored.",
			want:  StatusResolved,
		},
		{
			name:  "an update on an open incident is still an outage",
			title: "Elevated error rates",
			body:  "Update - We are continuing to investigate elevated error rates affecting Actions.",
			want:  StatusInvestigating,
		},
		{
			name:  "scheduled maintenance",
			title: "Database failover drill",
			body:  "Scheduled - We will be undergoing scheduled maintenance on 30 August.",
			want:  StatusMaintenance,
		},
		{
			name:  "maintenance under way",
			title: "Database failover drill",
			body:  "In progress - Scheduled maintenance is currently in progress.",
			want:  StatusMaintenance,
		},
		{
			// A finished maintenance window is over: nothing is wrong any more.
			name:  "completed maintenance reads as resolved",
			title: "Scheduled maintenance: database failover drill",
			body:  "Completed - The scheduled maintenance has been completed with no impact.",
			want:  StatusResolved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseStatus(tt.title, tt.body); got != tt.want {
				t.Errorf("ParseStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Statuspage concatenates an incident's updates newest-first, so the first marker in the
// body is the current state — later markers are history.
func TestParseStatusTakesTheFirstMarkerAsCurrent(t *testing.T) {
	body := "Monitoring - A fix has been applied.\n\n" +
		"Identified - The cause was a bad deploy.\n\n" +
		"Investigating - We are looking into reports of errors."

	if got := ParseStatus("Elevated errors", body); got != StatusMonitoring {
		t.Errorf("ParseStatus() = %q, want the newest update's status (monitoring)", got)
	}
}

func TestParseStatusIsCaseInsensitive(t *testing.T) {
	if got := ParseStatus("", "RESOLVED - all clear"); got != StatusResolved {
		t.Errorf("ParseStatus() = %q, want resolved", got)
	}
	if got := ParseStatus("", "resolved - all clear"); got != StatusResolved {
		t.Errorf("ParseStatus() = %q, want resolved", got)
	}
}

func TestParseStatusFallsBackToTheTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
		want  Status
	}{
		{
			name:  "maintenance declared in the title",
			title: "Scheduled maintenance: database failover drill",
			body:  "",
			want:  StatusMaintenance,
		},
		{
			name:  "status in the title when the body has none",
			title: "Resolved: increased error rates",
			body:  "We have restored service.",
			want:  StatusResolved,
		},
		{
			// The body wins when both say something.
			name:  "body beats title",
			title: "Scheduled maintenance",
			body:  "Completed - the window closed early.",
			want:  StatusResolved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseStatus(tt.title, tt.body); got != tt.want {
				t.Errorf("ParseStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Providers that don't follow the convention must resolve to unknown rather than to a
// guess — the UI renders that honestly as grey.
func TestParseStatusIsUnknownWhenNothingIsRecognised(t *testing.T) {
	tests := []struct {
		name  string
		title string
		body  string
	}{
		{"empty", "", ""},
		{
			name:  "azure-style prose",
			title: "Azure Front Door - Mitigation applied",
			body:  "Between 09:00 and 11:00 UTC a subset of customers may have experienced issues.",
		},
		{
			name:  "a plain announcement",
			title: "New region available",
			body:  "We have opened a new region in Warsaw.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseStatus(tt.title, tt.body); got != StatusUnknown {
				t.Errorf("ParseStatus() = %q, want unknown", got)
			}
		})
	}
}

// "resolved" appearing mid-sentence is prose, not a status marker; the marker form wins.
func TestParseStatusPrefersMarkerFormOverProse(t *testing.T) {
	body := "Investigating - reports that a previously resolved incident has returned."

	if got := ParseStatus("", body); got != StatusInvestigating {
		t.Errorf("ParseStatus() = %q, want investigating", got)
	}
}

func TestParseStatusAlwaysReturnsAValidStatus(t *testing.T) {
	inputs := []string{"", "Resolved - x", "gibberish", "Update - y", "In progress - z"}

	for _, in := range inputs {
		if got := ParseStatus(in, in); !got.Valid() {
			t.Errorf("ParseStatus(%q) = %q, which is not a valid Status", in, got)
		}
	}
}
