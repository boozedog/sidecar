package workspace

import (
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		wantErrs  bool // whether errors are expected
	}{
		{"valid simple", "feature-branch", true, false},
		{"valid with numbers", "feature-123", true, false},
		{"valid with underscore", "feature_branch", true, false},
		{"empty", "", false, false},
		{"starts with dash", "-feature", false, true},
		{"starts with dot", ".feature", false, true},
		{"ends with .lock", "feature.lock", false, true},
		{"contains space", "feature branch", false, true},
		{"contains tilde", "feature~branch", false, true},
		{"contains caret", "feature^branch", false, true},
		{"contains colon", "feature:branch", false, true},
		{"contains question", "feature?branch", false, true},
		{"contains asterisk", "feature*branch", false, true},
		{"contains bracket", "feature[branch", false, true},
		{"contains backslash", "feature\\branch", false, true},
		{"contains double dots", "feature..branch", false, true},
		{"contains @{", "feature@{branch", false, true},
		// Slash tests - important for branch prefix feature
		{"valid with single slash", "myrepo/feature", true, false},
		{"valid with multiple slashes", "org/repo/feature", true, false},
		{"starts with slash", "/feature", false, true},
		{"ends with slash", "feature/", false, true},
		{"double slash", "myrepo//feature", false, true},
		{"slash dot", "myrepo/.feature", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errs, _ := ValidateBranchName(tt.input)
			if valid != tt.wantValid {
				t.Errorf("ValidateBranchName(%q) valid = %v, want %v", tt.input, valid, tt.wantValid)
			}
			hasErrs := len(errs) > 0
			if hasErrs != tt.wantErrs {
				t.Errorf("ValidateBranchName(%q) hasErrors = %v, want %v, errors: %v", tt.input, hasErrs, tt.wantErrs, errs)
			}
		})
	}
}

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"spaces to dashes", "my feature", "my-feature"},
		{"removes tilde", "feature~1", "feature1"},
		{"removes caret", "feature^2", "feature2"},
		{"removes leading dash", "-feature", "feature"},
		{"removes leading dot", ".feature", "feature"},
		{"removes trailing .lock", "feature.lock", "feature"},
		{"removes trailing dot", "feature.", "feature"},
		{"removes trailing dash", "feature-", "feature"},
		{"lowercase", "MyFeature", "myfeature"},
		{"already clean", "feature-branch", "feature-branch"},
		{"complex", "My Feature~1^2", "my-feature12"},
		// Regression tests: .lock suffix exposed after trailing character cleanup
		{"lock-with-trailing-dash", "foo.lock-", "foo"},
		{"lock-with-trailing-dashes", "bar.lock--", "bar"},
		{"lock-with-trailing-slash", "branch.lock/", "branch"},
		{"lock-trailing-dash-multiple", "test.lock.lock-", "test"},
		// Slash handling for branch prefix feature
		{"preserves single slash", "myrepo/feature", "myrepo/feature"},
		{"removes leading slash", "/feature", "feature"},
		{"removes trailing slash", "feature/", "feature"},
		{"collapses double slash", "myrepo//feature", "myrepo/feature"},
		{"removes slash-dot", "myrepo/.feature", "myrepo/feature"},
		{"spaces in path with slash", "my repo/my feature", "my-repo/my-feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		mainWorkdir string
		wantCount   int
		wantNames   []string
		wantBranch  []string
		wantIsMain  []bool // Track which worktrees should be marked as main
	}{
		{
			name: "single worktree",
			output: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/project-feature
HEAD def456
branch refs/heads/feature
`,
			mainWorkdir: "/home/user/project",
			wantCount:   2, // Main + 1 worktree
			wantNames:   []string{"project", "project-feature"},
			wantBranch:  []string{"main", "feature"},
			wantIsMain:  []bool{true, false},
		},
		{
			name: "multiple worktrees",
			output: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/feature-a
HEAD def456
branch refs/heads/feature-a

worktree /home/user/feature-b
HEAD ghi789
branch refs/heads/feature-b
`,
			mainWorkdir: "/home/user/project",
			wantCount:   3, // Main + 2 worktrees
			wantNames:   []string{"project", "feature-a", "feature-b"},
			wantBranch:  []string{"main", "feature-a", "feature-b"},
			wantIsMain:  []bool{true, false, false},
		},
		{
			name: "detached head",
			output: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/detached
HEAD def456
detached
`,
			mainWorkdir: "/home/user/project",
			wantCount:   2, // Main + 1 worktree
			wantNames:   []string{"project", "detached"},
			wantBranch:  []string{"main", "(detached)"},
			wantIsMain:  []bool{true, false},
		},
		{
			name:        "empty output",
			output:      "",
			mainWorkdir: "/home/user/project",
			wantCount:   0,
			wantNames:   nil,
			wantBranch:  nil,
			wantIsMain:  nil,
		},
		// Branch prefix tests - branch name has repo prefix, directory name does not
		{
			name: "prefixed branch name",
			output: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/feature-auth
HEAD def456
branch refs/heads/project/feature-auth
`,
			mainWorkdir: "/home/user/project",
			wantCount:   2, // Main + 1 worktree
			wantNames:   []string{"project", "feature-auth"},
			wantBranch:  []string{"main", "project/feature-auth"},
			wantIsMain:  []bool{true, false},
		},
		{
			name: "multiple prefixed branches",
			output: `worktree /home/user/sidecar
HEAD abc123
branch refs/heads/main

worktree /home/user/fix-bug
HEAD def456
branch refs/heads/sidecar/fix-bug

worktree /home/user/add-feature
HEAD ghi789
branch refs/heads/sidecar/add-feature
`,
			mainWorkdir: "/home/user/sidecar",
			wantCount:   3, // Main + 2 worktrees
			wantNames:   []string{"sidecar", "fix-bug", "add-feature"},
			wantBranch:  []string{"main", "sidecar/fix-bug", "sidecar/add-feature"},
			wantIsMain:  []bool{true, false, false},
		},
		// Nested worktree directories - when branch name contains '/' and creates nested dirs
		{
			name: "nested worktree directory",
			output: `worktree /home/user/sidecar
HEAD abc123
branch refs/heads/main

worktree /home/user/sidecar-prefix/nested-branch
HEAD def456
branch refs/heads/nested-branch
`,
			mainWorkdir: "/home/user/sidecar",
			wantCount:   2, // Main + 1 worktree
			// Main uses basename, worktrees use full relative path
			wantNames:  []string{"sidecar", "sidecar-prefix/nested-branch"},
			wantBranch: []string{"main", "nested-branch"},
			wantIsMain: []bool{true, false},
		},
		{
			name: "deeply nested worktree directory",
			output: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/project-td-123/feature/auth/login
HEAD def456
branch refs/heads/feature/auth/login
`,
			mainWorkdir: "/home/user/project",
			wantCount:   2, // Main + 1 worktree
			// Full relative path from parent dir for worktrees
			wantNames:  []string{"project", "project-td-123/feature/auth/login"},
			wantBranch: []string{"main", "feature/auth/login"},
			wantIsMain: []bool{true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktrees, err := parseWorktreeList(tt.output, tt.mainWorkdir)
			if err != nil {
				t.Fatalf("parseWorktreeList() error = %v", err)
			}

			if len(worktrees) != tt.wantCount {
				t.Errorf("got %d worktrees, want %d", len(worktrees), tt.wantCount)
			}

			for i, wt := range worktrees {
				if i < len(tt.wantNames) && wt.Name != tt.wantNames[i] {
					t.Errorf("worktree[%d].Name = %q, want %q", i, wt.Name, tt.wantNames[i])
				}
				if i < len(tt.wantBranch) && wt.Branch != tt.wantBranch[i] {
					t.Errorf("worktree[%d].Branch = %q, want %q", i, wt.Branch, tt.wantBranch[i])
				}
				if i < len(tt.wantIsMain) && wt.IsMain != tt.wantIsMain[i] {
					t.Errorf("worktree[%d].IsMain = %v, want %v", i, wt.IsMain, tt.wantIsMain[i])
				}
			}
		})
	}
}

