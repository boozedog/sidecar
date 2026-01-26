package tty

import "testing"

func TestDetectBracketedPasteMode(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "enabled",
			output: "some output\x1b[?2004h more output",
			want:   true,
		},
		{
			name:   "disabled",
			output: "some output\x1b[?2004l more output",
			want:   false,
		},
		{
			name:   "enabled_then_disabled",
			output: "\x1b[?2004h some output \x1b[?2004l",
			want:   false,
		},
		{
			name:   "disabled_then_enabled",
			output: "\x1b[?2004l some output \x1b[?2004h",
			want:   true,
		},
		{
			name:   "no_sequences",
			output: "plain text output",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectBracketedPasteMode(tt.output); got != tt.want {
				t.Errorf("DetectBracketedPasteMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectMouseReportingMode(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "mode_1000_enabled",
			output: "\x1b[?1000h",
			want:   true,
		},
		{
			name:   "mode_1002_enabled",
			output: "\x1b[?1002h",
			want:   true,
		},
		{
			name:   "mode_1003_enabled",
			output: "\x1b[?1003h",
			want:   true,
		},
		{
			name:   "mode_1006_enabled",
			output: "\x1b[?1006h",
			want:   true,
		},
		{
			name:   "mode_1000_disabled",
			output: "\x1b[?1000l",
			want:   false,
		},
		{
			name:   "enabled_then_disabled",
			output: "\x1b[?1000h some output \x1b[?1000l",
			want:   false,
		},
		{
			name:   "disabled_then_enabled",
			output: "\x1b[?1000l some output \x1b[?1006h",
			want:   true,
		},
		{
			name:   "no_sequences",
			output: "plain text output",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectMouseReportingMode(tt.output); got != tt.want {
				t.Errorf("DetectMouseReportingMode() = %v, want %v", got, tt.want)
			}
		})
	}
}
