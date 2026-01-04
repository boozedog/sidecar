package gitstatus

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCommitAsMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		commit   *Commit
		contains []string
	}{
		{
			name: "basic commit",
			commit: &Commit{
				Hash:        "abc123456789",
				ShortHash:   "abc1234",
				Author:      "Test Author",
				AuthorEmail: "test@example.com",
				Date:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Subject:     "Fix important bug",
			},
			contains: []string{
				"# Fix important bug",
				"**Commit:** `abc1234`",
				"**Author:** Test Author <test@example.com>",
				"**Date:** 2024-01-15",
			},
		},
		{
			name: "commit with stats",
			commit: &Commit{
				Hash:        "def456789012",
				ShortHash:   "def4567",
				Author:      "Developer",
				AuthorEmail: "dev@example.com",
				Date:        time.Date(2024, 2, 20, 14, 0, 0, 0, time.UTC),
				Subject:     "Add new feature",
				Stats: CommitStats{
					FilesChanged: 3,
					Additions:    50,
					Deletions:    10,
				},
			},
			contains: []string{
				"# Add new feature",
				"**Stats:** 3 file(s), +50/-10",
			},
		},
		{
			name: "commit with body",
			commit: &Commit{
				Hash:        "ghi789012345",
				ShortHash:   "ghi7890",
				Author:      "Author",
				AuthorEmail: "author@example.com",
				Date:        time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC),
				Subject:     "Refactor module",
				Body:        "This refactors the core module for better performance.",
			},
			contains: []string{
				"# Refactor module",
				"## Message",
				"This refactors the core module for better performance.",
			},
		},
		{
			name: "commit with files",
			commit: &Commit{
				Hash:        "jkl012345678",
				ShortHash:   "jkl0123",
				Author:      "Dev",
				AuthorEmail: "dev@example.com",
				Date:        time.Date(2024, 4, 5, 16, 30, 0, 0, time.UTC),
				Subject:     "Update files",
				Files: []CommitFile{
					{Path: "file1.go", Status: StatusModified, Additions: 10, Deletions: 5},
					{Path: "file2.go", Status: StatusAdded, Additions: 20, Deletions: 0},
					{Path: "file3.go", Status: StatusDeleted, Additions: 0, Deletions: 15},
				},
			},
			contains: []string{
				"# Update files",
				"## Files Changed",
				"[M] `file1.go` (+10/-5)",
				"[A] `file2.go` (+20/-0)",
				"[D] `file3.go` (+0/-15)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCommitAsMarkdown(tt.commit)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected markdown to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestFileStatusIcon(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{StatusAdded, "[A]"},
		{StatusModified, "[M]"},
		{StatusDeleted, "[D]"},
		{StatusRenamed, "[R]"},
		{StatusCopied, "[C]"},
		{StatusUntracked, "[?]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := fileStatusIcon(tt.status)
			if result != tt.expected {
				t.Errorf("fileStatusIcon(%q) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestGetCurrentCommit(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Plugin)
		expected string // expected short hash, empty if nil
	}{
		{
			name: "status view with preview commit",
			setup: func(p *Plugin) {
				p.viewMode = ViewModeStatus
				p.activePane = PaneDiff
				p.previewCommit = &Commit{ShortHash: "eee5555"}
			},
			expected: "eee5555",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				tree: NewFileTree("/tmp"),
			}
			tt.setup(p)

			commit := p.getCurrentCommit()

			if tt.expected == "" {
				if commit != nil {
					t.Errorf("expected nil commit, got %+v", commit)
				}
			} else {
				if commit == nil {
					t.Errorf("expected commit with hash %q, got nil", tt.expected)
				} else if commit.ShortHash != tt.expected {
					t.Errorf("expected hash %q, got %q", tt.expected, commit.ShortHash)
				}
			}
		})
	}
}
