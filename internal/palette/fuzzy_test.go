package palette

import (
	"testing"

	"github.com/sst/sidecar/internal/plugin"
)

func TestFuzzyMatch_EmptyQuery(t *testing.T) {
	score, ranges := FuzzyMatch("", "stage-file")
	if score != 0 {
		t.Errorf("empty query should return 0 score, got %d", score)
	}
	if ranges != nil {
		t.Errorf("empty query should return nil ranges, got %v", ranges)
	}
}

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	score, ranges := FuzzyMatch("stage", "stage")
	if score <= 0 {
		t.Errorf("exact match should have positive score, got %d", score)
	}
	if len(ranges) != 1 || ranges[0].Start != 0 || ranges[0].End != 5 {
		t.Errorf("exact match should have single range [0,5], got %v", ranges)
	}
}

func TestFuzzyMatch_PartialMatch(t *testing.T) {
	score, ranges := FuzzyMatch("stg", "stage")
	if score <= 0 {
		t.Errorf("partial match should have positive score, got %d", score)
	}
	if len(ranges) == 0 {
		t.Errorf("partial match should have ranges, got none")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	score, ranges := FuzzyMatch("xyz", "stage")
	if score != 0 {
		t.Errorf("no match should return 0 score, got %d", score)
	}
	if ranges != nil {
		t.Errorf("no match should return nil ranges, got %v", ranges)
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	score1, _ := FuzzyMatch("STAGE", "stage")
	score2, _ := FuzzyMatch("stage", "STAGE")
	if score1 <= 0 || score2 <= 0 {
		t.Errorf("case insensitive match should work, got scores %d, %d", score1, score2)
	}
}

func TestFuzzyMatch_WordStartBonus(t *testing.T) {
	// "sf" matching "stage-file" should get bonus for matching word starts
	score1, _ := FuzzyMatch("sf", "stage-file")
	// "tf" matching "stage-file" shouldn't get as much bonus
	score2, _ := FuzzyMatch("tf", "stage-file")
	if score1 <= score2 {
		t.Errorf("word start matches should score higher: sf=%d, tf=%d", score1, score2)
	}
}

func TestFuzzyMatch_ConsecutiveBonus(t *testing.T) {
	// "sta" has 3 consecutive matches
	score1, _ := FuzzyMatch("sta", "stage")
	// "sae" has no consecutive matches
	score2, _ := FuzzyMatch("sae", "stage")
	if score1 <= score2 {
		t.Errorf("consecutive matches should score higher: sta=%d, sae=%d", score1, score2)
	}
}

func TestScoreEntry_EmptyQuery(t *testing.T) {
	entry := PaletteEntry{
		Name:        "Stage",
		Description: "Stage file",
		Key:         "s",
		Category:    plugin.CategoryGit,
	}
	ScoreEntry(&entry, "")
	if entry.Score != 0 {
		t.Errorf("empty query should give 0 score, got %d", entry.Score)
	}
}

func TestScoreEntry_NameMatch(t *testing.T) {
	entry := PaletteEntry{
		Name:        "Stage",
		Description: "Stage selected file",
		Key:         "s",
		Category:    plugin.CategoryGit,
	}
	ScoreEntry(&entry, "sta")
	if entry.Score <= 0 {
		t.Errorf("name match should give positive score, got %d", entry.Score)
	}
	if len(entry.MatchRanges) == 0 {
		t.Errorf("name match should populate MatchRanges")
	}
}

func TestScoreEntry_KeyMatch(t *testing.T) {
	entry := PaletteEntry{
		Name:        "Stage",
		Description: "Stage selected file",
		Key:         "ctrl+s",
		Category:    plugin.CategoryGit,
	}
	ScoreEntry(&entry, "ctrl")
	if entry.Score <= 0 {
		t.Errorf("key match should give positive score, got %d", entry.Score)
	}
}

func TestScoreEntry_LayerBoost(t *testing.T) {
	entryCurrent := PaletteEntry{
		Name:  "Stage",
		Layer: LayerCurrentMode,
	}
	entryPlugin := PaletteEntry{
		Name:  "Stage",
		Layer: LayerPlugin,
	}
	entryGlobal := PaletteEntry{
		Name:  "Stage",
		Layer: LayerGlobal,
	}

	ScoreEntry(&entryCurrent, "sta")
	ScoreEntry(&entryPlugin, "sta")
	ScoreEntry(&entryGlobal, "sta")

	if entryCurrent.Score <= entryPlugin.Score {
		t.Errorf("current mode should score higher than plugin: %d vs %d",
			entryCurrent.Score, entryPlugin.Score)
	}
	if entryPlugin.Score <= entryGlobal.Score {
		t.Errorf("plugin should score higher than global: %d vs %d",
			entryPlugin.Score, entryGlobal.Score)
	}
}

func TestFilterEntries_EmptyQuery(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "Stage", Layer: LayerPlugin},
		{Name: "Commit", Layer: LayerGlobal},
		{Name: "Diff", Layer: LayerCurrentMode},
	}

	filtered := FilterEntries(entries, "")
	if len(filtered) != 3 {
		t.Errorf("empty query should return all entries, got %d", len(filtered))
	}
	// Should be sorted by layer
	if filtered[0].Layer != LayerCurrentMode {
		t.Errorf("first entry should be current mode, got layer %d", filtered[0].Layer)
	}
}

func TestFilterEntries_WithQuery(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "Stage", Description: "Stage file"},
		{Name: "Push", Description: "Push changes"},
		{Name: "Status", Description: "Show status"},
	}

	filtered := FilterEntries(entries, "sta")
	// "sta" matches Stage and Status (both have s, t, a in order)
	if len(filtered) < 2 {
		t.Errorf("'sta' should match at least Stage and Status, got %d", len(filtered))
	}
	// First result should be Stage (better consecutive match)
	if filtered[0].Name != "Stage" {
		t.Errorf("first result should be 'Stage', got %q", filtered[0].Name)
	}
}

func TestFilterEntries_NoMatches(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "Push"},
		{Name: "Pull"},
	}

	filtered := FilterEntries(entries, "xyz")
	if len(filtered) != 0 {
		t.Errorf("'xyz' should match nothing, got %d", len(filtered))
	}
}

func TestSortEntries_ByScore(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "Low", Score: 10},
		{Name: "High", Score: 100},
		{Name: "Medium", Score: 50},
	}

	SortEntries(entries)

	if entries[0].Name != "High" || entries[1].Name != "Medium" || entries[2].Name != "Low" {
		t.Errorf("entries should be sorted by score descending, got %v", entries)
	}
}

func TestSortEntries_ByLayerWhenScoreEqual(t *testing.T) {
	entries := []PaletteEntry{
		{Name: "Global", Score: 50, Layer: LayerGlobal},
		{Name: "Current", Score: 50, Layer: LayerCurrentMode},
		{Name: "Plugin", Score: 50, Layer: LayerPlugin},
	}

	SortEntries(entries)

	if entries[0].Layer != LayerCurrentMode {
		t.Errorf("current mode should come first when scores equal")
	}
	if entries[1].Layer != LayerPlugin {
		t.Errorf("plugin should come second when scores equal")
	}
	if entries[2].Layer != LayerGlobal {
		t.Errorf("global should come last when scores equal")
	}
}
