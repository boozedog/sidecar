package app

import "testing"

func TestParseReleaseNotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no headers to strip",
			input:    "#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "strips ## What's New header",
			input:    "## What's New\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "strips ### What's New header",
			input:    "### What's New\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "strips # What's New header",
			input:    "# What's New\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "case insensitive",
			input:    "## WHAT'S NEW\n\n#### Bug Fixes",
			expected: "#### Bug Fixes",
		},
		{
			name:     "strips multiple duplicate headers",
			input:    "## What's New\n\n### What's New\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "strips leading whitespace",
			input:    "\n\n## What's New\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "collapses multiple newlines",
			input:    "#### Bug Fixes\n\n\n\n- Minor fixes",
			expected: "#### Bug Fixes\n\n- Minor fixes",
		},
		{
			name:     "strips Release Notes header",
			input:    "## Release Notes\n\n#### Bug Fixes\n- Minor fixes",
			expected: "#### Bug Fixes\n- Minor fixes",
		},
		{
			name:     "preserves content when only headers stripped",
			input:    "## What's New\n",
			expected: "## What's New",
		},
		{
			name: "real world example",
			input: `

## What's New

### What's New

#### Bug Fixes
- Minor fixes to conversation search
`,
			expected: `#### Bug Fixes
- Minor fixes to conversation search`,
		},
		{
			name:     "whats without apostrophe",
			input:    "## Whats New\n\n#### Features",
			expected: "#### Features",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseReleaseNotes(tt.input)
			if got != tt.expected {
				t.Errorf("parseReleaseNotes() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}
