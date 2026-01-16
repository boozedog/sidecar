package app

import "testing"

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []WorktreeInfo
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name: "single worktree (main)",
			input: `worktree /Users/test/project
HEAD abc123def456
branch refs/heads/main

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
			},
		},
		{
			name: "main worktree plus linked worktree",
			input: `worktree /Users/test/project
HEAD abc123def456
branch refs/heads/main

worktree /Users/test/project-feature
HEAD def789abc012
branch refs/heads/feature-auth

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
				{Path: "/Users/test/project-feature", Branch: "feature-auth", IsMain: false},
			},
		},
		{
			name: "multiple worktrees",
			input: `worktree /main/repo
HEAD abc123
branch refs/heads/main

worktree /worktree/feature-a
HEAD def456
branch refs/heads/feature-a

worktree /worktree/feature-b
HEAD ghi789
branch refs/heads/feature-b

`,
			expected: []WorktreeInfo{
				{Path: "/main/repo", Branch: "main", IsMain: true},
				{Path: "/worktree/feature-a", Branch: "feature-a", IsMain: false},
				{Path: "/worktree/feature-b", Branch: "feature-b", IsMain: false},
			},
		},
		{
			name: "detached HEAD worktree",
			input: `worktree /Users/test/project
HEAD abc123def456
branch refs/heads/main

worktree /Users/test/project-detached
HEAD def789abc012
detached

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
				{Path: "/Users/test/project-detached", Branch: "", IsMain: false},
			},
		},
		{
			name: "no trailing newline",
			input: `worktree /Users/test/project
HEAD abc123def456
branch refs/heads/main`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
			},
		},
		{
			name: "nested branch name",
			input: `worktree /Users/test/project
HEAD abc123def456
branch refs/heads/feature/nested/branch

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "feature/nested/branch", IsMain: true},
			},
		},
		{
			name: "path with trailing slash is cleaned",
			input: `worktree /Users/test/project/
HEAD abc123def456
branch refs/heads/main

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
			},
		},
		{
			name: "path with relative components is cleaned",
			input: `worktree /Users/test/../test/project
HEAD abc123def456
branch refs/heads/main

`,
			expected: []WorktreeInfo{
				{Path: "/Users/test/project", Branch: "main", IsMain: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseWorktreeList(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d worktrees, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i].Path != expected.Path {
					t.Errorf("worktree[%d].Path = %q, want %q", i, result[i].Path, expected.Path)
				}
				if result[i].Branch != expected.Branch {
					t.Errorf("worktree[%d].Branch = %q, want %q", i, result[i].Branch, expected.Branch)
				}
				if result[i].IsMain != expected.IsMain {
					t.Errorf("worktree[%d].IsMain = %v, want %v", i, result[i].IsMain, expected.IsMain)
				}
			}
		})
	}
}

func TestParseRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"git@github.com:user/repo.git", "repo"},
		{"git@github.com:user/repo", "repo"},
		{"https://github.com/user/repo.git", "repo"},
		{"https://github.com/user/repo", "repo"},
		{"ssh://git@github.com/user/repo.git", "repo"},
		{"https://gitlab.com/group/subgroup/repo.git", "repo"},
		{"git@gitlab.com:group/subgroup/repo.git", "repo"},
		{"simple-name", "simple-name"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := parseRepoNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("parseRepoNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}
