package tty

import (
	"testing"
	"time"
)

func TestCalculatePollingInterval(t *testing.T) {
	tests := []struct {
		name           string
		inactivityTime time.Duration
		want           time.Duration
	}{
		{
			name:           "active_typing",
			inactivityTime: 100 * time.Millisecond,
			want:           PollingDecayFast,
		},
		{
			name:           "brief_inactivity",
			inactivityTime: 3 * time.Second,
			want:           PollingDecayMedium,
		},
		{
			name:           "extended_inactivity",
			inactivityTime: 15 * time.Second,
			want:           PollingDecaySlow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lastActivity := time.Now().Add(-tt.inactivityTime)
			got := CalculatePollingInterval(lastActivity)
			if got != tt.want {
				t.Errorf("CalculatePollingInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPollingConstants(t *testing.T) {
	// Verify constants are set to reasonable values
	if PollingDecayFast >= PollingDecayMedium {
		t.Error("PollingDecayFast should be less than PollingDecayMedium")
	}
	if PollingDecayMedium >= PollingDecaySlow {
		t.Error("PollingDecayMedium should be less than PollingDecaySlow")
	}
	if DoubleEscapeDelay <= 0 {
		t.Error("DoubleEscapeDelay should be positive")
	}
}
