// Package domain holds the core entities, value objects, and (later) port
// interfaces for the status-page reader. It imports only the standard library.
package domain

// Status is the normalized, best-effort state of an incident, derived from the
// provider-specific text of a feed entry. It is never authoritative: feeds that
// don't follow the Atlassian Statuspage convention (e.g. Azure, incident.io) will
// often resolve to StatusUnknown, in which case the raw content is the source of truth.
type Status string

// The incident states we recognize, mirroring the Status enum in api/openapi.yaml.
const (
	StatusInvestigating Status = "investigating"
	StatusIdentified    Status = "identified"
	StatusMonitoring    Status = "monitoring"
	StatusResolved      Status = "resolved"
	StatusMaintenance   Status = "maintenance"
	StatusUnknown       Status = "unknown"
)

// Valid reports whether s is one of the known Status values.
func (s Status) Valid() bool {
	switch s {
	case StatusInvestigating, StatusIdentified, StatusMonitoring,
		StatusResolved, StatusMaintenance, StatusUnknown:
		return true
	default:
		return false
	}
}
