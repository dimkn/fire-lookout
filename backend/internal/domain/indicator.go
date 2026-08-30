package domain

// Indicator is the traffic-light a system is shown with. It is a coarse projection of
// Status: several incident states collapse into one colour, and anything we cannot
// interpret stays IndicatorUnknown rather than pretending to be good news.
type Indicator string

// The four lights a system can show, mirroring the Indicator enum in api/openapi.yaml.
const (
	IndicatorOperational Indicator = "operational" // green
	IndicatorDegraded    Indicator = "degraded"    // yellow
	IndicatorOutage      Indicator = "outage"      // red
	IndicatorUnknown     Indicator = "unknown"     // grey
)

// Indicator maps an incident state onto the traffic-light. Unrecognized values —
// including the empty string and provider-specific states we never learned to parse —
// deliberately resolve to IndicatorUnknown.
func (s Status) Indicator() Indicator {
	switch s {
	case StatusInvestigating, StatusIdentified:
		return IndicatorOutage
	case StatusMonitoring, StatusMaintenance:
		return IndicatorDegraded
	case StatusResolved:
		return IndicatorOperational
	case StatusUnknown:
		return IndicatorUnknown
	default:
		return IndicatorUnknown
	}
}
