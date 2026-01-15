package gitstatus

import (
	"testing"

	"github.com/marcus/sidecar/internal/mouse"
)

func makeEntries(count int, status FileStatus) []*FileEntry {
	entries := make([]*FileEntry, count)
	for i := 0; i < count; i++ {
		entries[i] = &FileEntry{
			Path:   "file",
			Status: status,
		}
	}
	return entries
}

func TestCommitSectionCapacity_TruncatesFiles(t *testing.T) {
	p := &Plugin{
		tree: &FileTree{
			Staged:    makeEntries(2, StatusAdded),
			Modified:  makeEntries(3, StatusModified),
			Untracked: makeEntries(2, StatusUntracked),
		},
		recentCommits: make([]*Commit, 10),
	}

	got := p.commitSectionCapacity(16)
	want := 5
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}

func TestCommitSectionCapacity_CleanWithStatus(t *testing.T) {
	p := &Plugin{
		tree:           &FileTree{},
		pushInProgress: true,
	}

	got := p.commitSectionCapacity(10)
	want := 5
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}

func TestCommitSectionCapacity_Minimum(t *testing.T) {
	p := &Plugin{
		tree: &FileTree{},
	}

	got := p.commitSectionCapacity(5)
	want := 2
	if got != want {
		t.Fatalf("commitSectionCapacity = %d, want %d", got, want)
	}
}

func makeCommitsWithHash(count int) []*Commit {
	commits := make([]*Commit, count)
	for i := range count {
		commits[i] = &Commit{
			Hash:    "abc1234" + string(rune('a'+i)),
			Subject: "Test commit",
			Pushed:  true,
		}
	}
	return commits
}

func TestCommitHitRegions_CleanTree(t *testing.T) {
	// When tree is clean ("Working tree clean" shown), commit hit regions
	// must align with visual positions. This tests the fix for the off-by-one
	// bug where clicking a commit row selected the row above.
	handler := mouse.NewHandler()
	p := &Plugin{
		tree:          &FileTree{}, // Empty = "Working tree clean"
		sidebarWidth:  40,
		mouseHandler:  handler,
		recentCommits: makeCommitsWithHash(5),
	}

	// Render sidebar to populate hit regions
	_ = p.renderSidebar(20)

	// Find all commit regions and verify they have sequential indices
	var commitRegions []mouse.Region
	for _, r := range handler.HitMap.Regions() {
		if r.ID == regionCommit {
			commitRegions = append(commitRegions, r)
		}
	}

	if len(commitRegions) == 0 {
		t.Fatal("no commit hit regions registered")
	}

	// Verify regions have correct data indices (0, 1, 2, ...)
	for i, r := range commitRegions {
		idx, ok := r.Data.(int)
		if !ok {
			t.Fatalf("region %d has non-int data: %T", i, r.Data)
		}
		if idx != i {
			t.Fatalf("region %d has data=%d, want %d", i, idx, i)
		}
	}

	// Verify hit test at first commit Y returns commit 0
	firstCommitY := commitRegions[0].Rect.Y
	region := handler.HitMap.Test(5, firstCommitY)
	if region == nil {
		t.Fatalf("hit test at Y=%d returned nil", firstCommitY)
	}
	if region.ID != regionCommit {
		t.Fatalf("hit test returned region %q, want %q", region.ID, regionCommit)
	}
	if idx, _ := region.Data.(int); idx != 0 {
		t.Fatalf("hit test returned commit %d, want 0", idx)
	}
}

// TestToggleSidebar tests the sidebar toggle focus restoration behavior.
func TestToggleSidebar(t *testing.T) {
	tests := []struct {
		name            string
		startPane       FocusPane
		expectedRestore FocusPane
	}{
		{
			name:            "sidebar to sidebar",
			startPane:       PaneSidebar,
			expectedRestore: PaneSidebar,
		},
		{
			name:            "diff to diff",
			startPane:       PaneDiff,
			expectedRestore: PaneDiff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				activePane:     tt.startPane,
				sidebarVisible: true,
				sidebarRestore: PaneSidebar,
			}

			// Collapse sidebar
			p.toggleSidebar()

			if p.sidebarVisible {
				t.Error("sidebar should be hidden after collapse")
			}
			if p.sidebarRestore != tt.startPane {
				t.Errorf("sidebarRestore = %d, want %d", p.sidebarRestore, tt.startPane)
			}
			// When collapsed from sidebar, focus should move to diff
			if tt.startPane == PaneSidebar && p.activePane != PaneDiff {
				t.Errorf("activePane should be PaneDiff after collapsing from sidebar, got %d", p.activePane)
			}

			// Expand sidebar
			p.toggleSidebar()

			if !p.sidebarVisible {
				t.Error("sidebar should be visible after expand")
			}
			if p.activePane != tt.expectedRestore {
				t.Errorf("activePane = %d, want %d after restore", p.activePane, tt.expectedRestore)
			}
		})
	}
}

// TestToggleSidebarRapidToggle tests multiple rapid toggles don't corrupt state.
func TestToggleSidebarRapidToggle(t *testing.T) {
	p := &Plugin{
		activePane:     PaneDiff,
		sidebarVisible: true,
		sidebarRestore: PaneSidebar,
	}

	// Toggle 5 times rapidly
	for i := 0; i < 5; i++ {
		p.toggleSidebar()
	}

	// After odd number of toggles, sidebar should be hidden
	if p.sidebarVisible {
		t.Error("sidebar should be hidden after odd number of toggles")
	}

	// Toggle once more to show
	p.toggleSidebar()

	if !p.sidebarVisible {
		t.Error("sidebar should be visible")
	}
	// Should restore to PaneDiff
	if p.activePane != PaneDiff {
		t.Errorf("activePane should restore to PaneDiff, got %d", p.activePane)
	}
}

// TestToggleSidebarCollapseFromDiff verifies focus stays on diff when collapsing from diff.
func TestToggleSidebarCollapseFromDiff(t *testing.T) {
	p := &Plugin{
		activePane:     PaneDiff,
		sidebarVisible: true,
		sidebarRestore: PaneSidebar,
	}

	p.toggleSidebar()

	// Focus should stay on diff (since we weren't on sidebar)
	if p.activePane != PaneDiff {
		t.Errorf("activePane should remain PaneDiff, got %d", p.activePane)
	}
}

// TestSidebarRestoreInitialization verifies sidebarRestore is initialized correctly in New().
func TestSidebarRestoreInitialization(t *testing.T) {
	p := New()

	if p.sidebarRestore != PaneSidebar {
		t.Errorf("sidebarRestore should default to PaneSidebar, got %d", p.sidebarRestore)
	}
}

// TestFolderTriangleOnlyWithChildren verifies expand triangles only show for folders with children.
func TestFolderTriangleOnlyWithChildren(t *testing.T) {
	tests := []struct {
		name           string
		entry          *FileEntry
		expectTriangle bool
	}{
		{
			name: "folder with children shows triangle",
			entry: &FileEntry{
				Path:     "folder/",
				Status:   StatusUntracked,
				IsFolder: true,
				Children: []*FileEntry{
					{Path: "folder/file1.txt", Status: StatusUntracked},
					{Path: "folder/file2.txt", Status: StatusUntracked},
				},
			},
			expectTriangle: true,
		},
		{
			name: "folder with no children shows no triangle",
			entry: &FileEntry{
				Path:     "empty-folder/",
				Status:   StatusUntracked,
				IsFolder: true,
				Children: []*FileEntry{}, // Empty children
			},
			expectTriangle: false,
		},
		{
			name: "folder with nil children shows no triangle",
			entry: &FileEntry{
				Path:     "nil-folder/",
				Status:   StatusUntracked,
				IsFolder: true,
				Children: nil, // Nil children
			},
			expectTriangle: false,
		},
		{
			name: "regular file shows no triangle",
			entry: &FileEntry{
				Path:     "file.txt",
				Status:   StatusUntracked,
				IsFolder: false,
			},
			expectTriangle: false,
		},
	}

	p := &Plugin{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.renderSidebarEntry(tt.entry, false, 40)

			hasCollapsedTriangle := containsString(result, "▶")
			hasExpandedTriangle := containsString(result, "▼")
			hasTriangle := hasCollapsedTriangle || hasExpandedTriangle

			if tt.expectTriangle && !hasTriangle {
				t.Errorf("expected triangle in output for %q, got: %s", tt.name, result)
			}
			if !tt.expectTriangle && hasTriangle {
				t.Errorf("did not expect triangle in output for %q, got: %s", tt.name, result)
			}
		})
	}
}

// containsString checks if the styled string contains the given substring.
// Strips ANSI codes for reliable string matching.
func containsString(s, substr string) bool {
	// Simple approach: check for the substring directly
	// ANSI codes won't contain our triangle characters
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
