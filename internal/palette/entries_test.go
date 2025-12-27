package palette

import (
	"testing"

	"github.com/sst/sidecar/internal/plugin"
)

func TestLayerName(t *testing.T) {
	tests := []struct {
		layer Layer
		want  string
	}{
		{LayerCurrentMode, "Current"},
		{LayerPlugin, "Plugin"},
		{LayerGlobal, "Global"},
		{Layer(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.layer.Name()
		if got != tt.want {
			t.Errorf("Layer(%d).Name() = %q, want %q", tt.layer, got, tt.want)
		}
	}
}

func TestDetermineLayer_CurrentMode(t *testing.T) {
	layer := determineLayer("git-diff", "git-diff", "git-status")
	if layer != LayerCurrentMode {
		t.Errorf("binding context matching active should be CurrentMode, got %d", layer)
	}
}

func TestDetermineLayer_Plugin(t *testing.T) {
	layer := determineLayer("git-status", "git-diff", "git-status")
	if layer != LayerPlugin {
		t.Errorf("binding context matching plugin should be Plugin, got %d", layer)
	}
}

func TestDetermineLayer_Global(t *testing.T) {
	layer := determineLayer("global", "git-diff", "git-status")
	if layer != LayerGlobal {
		t.Errorf("global context should be Global, got %d", layer)
	}
}

func TestFormatCommandID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"stage-file", "Stage file"},
		{"commit", "Commit"},
		{"show-diff-staged", "Show diff staged"},
		{"", ""},
	}

	for _, tt := range tests {
		got := formatCommandID(tt.input)
		if got != tt.want {
			t.Errorf("formatCommandID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		cmdID string
		want  plugin.Category
	}{
		{"scroll-down", plugin.CategoryNavigation},
		{"cursor-up", plugin.CategoryNavigation},
		{"next-item", plugin.CategoryNavigation},
		{"focus-plugin", plugin.CategoryNavigation},
		{"search-files", plugin.CategorySearch},
		{"find-match", plugin.CategorySearch},
		{"show-diff", plugin.CategoryView},
		{"toggle-mode", plugin.CategoryView},
		{"view-commit", plugin.CategoryView},
		{"stage-file", plugin.CategoryGit},
		{"commit-changes", plugin.CategoryGit},
		{"push-remote", plugin.CategoryGit},
		{"delete-item", plugin.CategoryEdit},
		{"add-file", plugin.CategoryEdit},
		{"quit", plugin.CategorySystem},
		{"refresh", plugin.CategorySystem},
		{"help", plugin.CategorySystem},
		{"unknown-command", plugin.CategoryActions},
	}

	for _, tt := range tests {
		got := inferCategory(tt.cmdID)
		if got != tt.want {
			t.Errorf("inferCategory(%q) = %q, want %q", tt.cmdID, got, tt.want)
		}
	}
}

func TestGroupEntriesByLayer(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "A", Layer: LayerCurrentMode},
		{Name: "B", Layer: LayerGlobal},
		{Name: "C", Layer: LayerPlugin},
		{Name: "D", Layer: LayerCurrentMode},
		{Name: "E", Layer: LayerGlobal},
	}

	groups := GroupEntriesByLayer(entries)

	if len(groups[LayerCurrentMode]) != 2 {
		t.Errorf("should have 2 current mode entries, got %d", len(groups[LayerCurrentMode]))
	}
	if len(groups[LayerPlugin]) != 1 {
		t.Errorf("should have 1 plugin entry, got %d", len(groups[LayerPlugin]))
	}
	if len(groups[LayerGlobal]) != 2 {
		t.Errorf("should have 2 global entries, got %d", len(groups[LayerGlobal]))
	}
}

func TestPaletteEntry_Fields(t *testing.T) {
	entry := PaletteEntry{
		Key:         "s",
		CommandID:   "stage-file",
		Name:        "Stage",
		Description: "Stage the selected file",
		Category:    plugin.CategoryGit,
		Context:     "git-status",
		Layer:       LayerPlugin,
		Score:       100,
		MatchRanges: []MatchRange{{Start: 0, End: 2}},
	}

	if entry.Key != "s" {
		t.Errorf("Key should be 's', got %q", entry.Key)
	}
	if entry.CommandID != "stage-file" {
		t.Errorf("CommandID should be 'stage-file', got %q", entry.CommandID)
	}
	if entry.Name != "Stage" {
		t.Errorf("Name should be 'Stage', got %q", entry.Name)
	}
	if entry.Description != "Stage the selected file" {
		t.Errorf("Description mismatch")
	}
	if entry.Category != plugin.CategoryGit {
		t.Errorf("Category should be Git, got %q", entry.Category)
	}
	if entry.Context != "git-status" {
		t.Errorf("Context should be 'git-status', got %q", entry.Context)
	}
	if entry.Layer != LayerPlugin {
		t.Errorf("Layer should be Plugin, got %d", entry.Layer)
	}
	if entry.Score != 100 {
		t.Errorf("Score should be 100, got %d", entry.Score)
	}
	if len(entry.MatchRanges) != 1 {
		t.Errorf("MatchRanges should have 1 element")
	}
}
