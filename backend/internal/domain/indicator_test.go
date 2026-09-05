package domain

import "testing"

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status Status
		want   Indicator
	}{
		{StatusInvestigating, IndicatorOutage},
		{StatusIdentified, IndicatorOutage},
		{StatusMonitoring, IndicatorDegraded},
		{StatusMaintenance, IndicatorDegraded},
		{StatusResolved, IndicatorOperational},
		{StatusUnknown, IndicatorUnknown},
		{Status(""), IndicatorUnknown},
		{Status("something-a-provider-invented"), IndicatorUnknown},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.Indicator(); got != tt.want {
				t.Errorf("Status(%q).Indicator() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
