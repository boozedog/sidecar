package worktree

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDefaultEnvOverrides(t *testing.T) {
	// GOWORK should be set to "off" to prevent workspace conflicts
	if v, ok := DefaultEnvOverrides["GOWORK"]; !ok || v != "off" {
		t.Errorf("GOWORK should be 'off', got %q", v)
	}

	// These should be cleared (empty string)
	cleared := []string{"GOFLAGS", "NODE_OPTIONS", "NODE_PATH", "PYTHONPATH", "VIRTUAL_ENV"}
	for _, key := range cleared {
		if v, ok := DefaultEnvOverrides[key]; !ok || v != "" {
			t.Errorf("%s should be cleared (empty), got %q", key, v)
		}
	}
}

func TestParseWorktreeEnvFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "empty file",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:    "simple key=value",
			content: "FOO=bar\nBAZ=qux",
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name:    "with comments",
			content: "# Comment\nFOO=bar\n# Another comment\nBAZ=qux",
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		{
			name:    "with quotes",
			content: "FOO=\"bar baz\"\nQUX='single quoted'",
			expected: map[string]string{
				"FOO": "bar baz",
				"QUX": "single quoted",
			},
		},
		{
			name:    "empty value",
			content: "CLEARED=",
			expected: map[string]string{
				"CLEARED": "",
			},
		},
		{
			name:    "with whitespace",
			content: "  FOO = bar  \n  BAZ=qux  ",
			expected: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create test file
			envPath := filepath.Join(tmpDir, worktreeEnvFile)
			if err := os.WriteFile(envPath, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := parseWorktreeEnvFile(tmpDir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tc.expected) {
				t.Errorf("expected %d entries, got %d: %v", len(tc.expected), len(result), result)
			}

			for k, v := range tc.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

func TestParseWorktreeEnvFile_NoFile(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := parseWorktreeEnvFile(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map for missing file, got %v", result)
	}
}

func TestBuildEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .worktree-env with custom override
	envPath := filepath.Join(tmpDir, worktreeEnvFile)
	content := "GOWORK=on\nCUSTOM_VAR=custom_value"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := BuildEnvOverrides(tmpDir)

	// User override should take precedence
	if result["GOWORK"] != "on" {
		t.Errorf("user override should take precedence: GOWORK=%q, expected 'on'", result["GOWORK"])
	}

	// Custom var should be present
	if result["CUSTOM_VAR"] != "custom_value" {
		t.Errorf("CUSTOM_VAR=%q, expected 'custom_value'", result["CUSTOM_VAR"])
	}

	// Default cleared vars should still be present
	if result["NODE_OPTIONS"] != "" {
		t.Errorf("NODE_OPTIONS should be cleared, got %q", result["NODE_OPTIONS"])
	}
}

func TestGenerateExportCommands(t *testing.T) {
	overrides := map[string]string{
		"GOWORK":  "off",
		"CLEARED": "",
		"QUOTED":  "has spaces",
	}

	cmds := GenerateExportCommands(overrides)

	// Sort for consistent comparison
	sort.Strings(cmds)

	expected := []string{
		"export GOWORK='off'",
		"export QUOTED='has spaces'",
		"unset CLEARED",
	}
	sort.Strings(expected)

	if len(cmds) != len(expected) {
		t.Fatalf("expected %d commands, got %d: %v", len(expected), len(cmds), cmds)
	}

	for i, cmd := range cmds {
		if cmd != expected[i] {
			t.Errorf("command %d: expected %q, got %q", i, expected[i], cmd)
		}
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	baseEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"GOWORK=/path/to/go.work",
		"NODE_OPTIONS=--max-old-space-size=4096",
	}

	overrides := map[string]string{
		"GOWORK":       "off",
		"NODE_OPTIONS": "", // Clear
		"NEW_VAR":      "new_value",
	}

	result := ApplyEnvOverrides(baseEnv, overrides)

	// Build map for easy checking
	resultMap := make(map[string]string)
	for _, entry := range result {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				resultMap[entry[:i]] = entry[i+1:]
				break
			}
		}
	}

	// PATH and HOME should be unchanged
	if resultMap["PATH"] != "/usr/bin" {
		t.Errorf("PATH should be unchanged, got %q", resultMap["PATH"])
	}
	if resultMap["HOME"] != "/home/user" {
		t.Errorf("HOME should be unchanged, got %q", resultMap["HOME"])
	}

	// GOWORK should be overridden
	if resultMap["GOWORK"] != "off" {
		t.Errorf("GOWORK should be 'off', got %q", resultMap["GOWORK"])
	}

	// NODE_OPTIONS should be removed (cleared)
	if _, ok := resultMap["NODE_OPTIONS"]; ok {
		t.Errorf("NODE_OPTIONS should be removed, but found %q", resultMap["NODE_OPTIONS"])
	}

	// NEW_VAR should be added
	if resultMap["NEW_VAR"] != "new_value" {
		t.Errorf("NEW_VAR should be 'new_value', got %q", resultMap["NEW_VAR"])
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with spaces", "'with spaces'"},
		{"with'quote", "'with'\"'\"'quote'"},
	}

	for _, tc := range tests {
		result := shellQuote(tc.input)
		if result != tc.expected {
			t.Errorf("shellQuote(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
