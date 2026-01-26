package tty

import (
	"time"
)

// Polling interval constants for adaptive polling
const (
	// PollingDecayFast is the polling interval during active typing.
	PollingDecayFast = 50 * time.Millisecond

	// PollingDecayMedium is the polling interval after brief inactivity.
	PollingDecayMedium = 200 * time.Millisecond

	// PollingDecaySlow is the polling interval after extended inactivity.
	PollingDecaySlow = 500 * time.Millisecond

	// KeystrokeDebounce delays polling after keystrokes to batch rapid typing.
	// Allows typing bursts to coalesce into fewer polls, reducing CPU usage.
	KeystrokeDebounce = 20 * time.Millisecond

	// InactivityMediumThreshold triggers medium polling.
	InactivityMediumThreshold = 2 * time.Second

	// InactivitySlowThreshold triggers slow polling.
	InactivitySlowThreshold = 10 * time.Second
)

// Interactive mode timing constants
const (
	// DoubleEscapeDelay is the max time between Escape presses for double-escape exit.
	// Single Escape is delayed by this amount to detect double-press.
	DoubleEscapeDelay = 150 * time.Millisecond
)

// CalculatePollingInterval determines the appropriate polling interval based on
// the time since the last user activity.
func CalculatePollingInterval(lastActivityTime time.Time) time.Duration {
	inactivity := time.Since(lastActivityTime)

	if inactivity > InactivitySlowThreshold {
		return PollingDecaySlow
	}
	if inactivity > InactivityMediumThreshold {
		return PollingDecayMedium
	}
	return PollingDecayFast
}
