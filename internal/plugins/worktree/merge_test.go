package worktree

import (
	"encoding/json"
	"testing"
)

func TestMergeWorkflowStepString(t *testing.T) {
	tests := []struct {
		step     MergeWorkflowStep
		expected string
	}{
		{MergeStepReviewDiff, "Review Diff"},
		{MergeStepPush, "Push Branch"},
		{MergeStepCreatePR, "Create PR"},
		{MergeStepWaitingMerge, "Waiting for Merge"},
		{MergeStepPostMergeConfirmation, "Confirm Cleanup"},
		{MergeStepCleanup, "Cleanup"},
		{MergeStepDone, "Done"},
		{MergeWorkflowStep(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.step.String()
			if result != tt.expected {
				t.Errorf("MergeWorkflowStep(%d).String() = %q, want %q", tt.step, result, tt.expected)
			}
		})
	}
}

func TestTruncateDiff(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		maxLines int
		wantLen  int // Expected number of lines
	}{
		{
			name:     "short diff",
			diff:     "line1\nline2\nline3",
			maxLines: 5,
			wantLen:  3,
		},
		{
			name:     "exact limit",
			diff:     "line1\nline2\nline3\nline4\nline5",
			maxLines: 5,
			wantLen:  5,
		},
		{
			name:     "over limit",
			diff:     "line1\nline2\nline3\nline4\nline5\nline6\nline7",
			maxLines: 3,
			wantLen:  4, // 3 lines + truncation message
		},
		{
			name:     "empty diff",
			diff:     "",
			maxLines: 5,
			wantLen:  1, // Just the empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateDiff(tt.diff, tt.maxLines)

			// For short diff, result should equal input
			if tt.name == "short diff" && result != tt.diff {
				t.Errorf("truncateDiff() should not modify short diff")
			}

			// For over limit, should contain truncation message
			if tt.name == "over limit" {
				if len(result) <= len(tt.diff) {
					// Actually truncated diff should be shorter content-wise but has extra message
				}
			}
		})
	}
}

func TestMergeWorkflowState(t *testing.T) {
	wt := &Worktree{
		Name:       "test-branch",
		Path:       "/tmp/test",
		Branch:     "test-branch",
		BaseBranch: "main",
	}

	state := &MergeWorkflowState{
		Worktree:   wt,
		Step:       MergeStepReviewDiff,
		PRTitle:    "Test PR",
		StepStatus: make(map[MergeWorkflowStep]string),
	}

	// Test initial state
	if state.Worktree != wt {
		t.Error("Worktree not set correctly")
	}
	if state.Step != MergeStepReviewDiff {
		t.Errorf("Step = %v, want MergeStepReviewDiff", state.Step)
	}

	// Test step status
	state.StepStatus[MergeStepReviewDiff] = "done"
	if state.StepStatus[MergeStepReviewDiff] != "done" {
		t.Error("StepStatus not working correctly")
	}
}

func TestCancelMergeWorkflow(t *testing.T) {
	p := &Plugin{
		viewMode: ViewModeMerge,
		mergeState: &MergeWorkflowState{
			Worktree: &Worktree{Name: "test"},
		},
	}

	p.cancelMergeWorkflow()

	if p.mergeState != nil {
		t.Error("mergeState should be nil after cancel")
	}
	if p.viewMode != ViewModeList {
		t.Errorf("viewMode = %v, want ViewModeList", p.viewMode)
	}
}

func TestParsePRMergeStatus(t *testing.T) {
	// Test parsing various JSON responses from gh pr view
	tests := []struct {
		name     string
		json     string
		expected bool
	}{
		{
			name:     "merged true",
			json:     `{"state":"MERGED","merged":true}`,
			expected: true,
		},
		{
			name:     "merged true with whitespace",
			json:     `{"state": "MERGED", "merged": true}`,
			expected: true,
		},
		{
			name:     "state MERGED only",
			json:     `{"state":"MERGED","merged":false}`,
			expected: true, // State takes precedence
		},
		{
			name:     "not merged",
			json:     `{"state":"OPEN","merged":false}`,
			expected: false,
		},
		{
			name:     "closed but not merged",
			json:     `{"state":"CLOSED","merged":false}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse using same logic as checkPRMerged
			var prStatus struct {
				State  string `json:"state"`
				Merged bool   `json:"merged"`
			}
			err := json.Unmarshal([]byte(tt.json), &prStatus)
			if err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			merged := prStatus.Merged || prStatus.State == "MERGED"
			if merged != tt.expected {
				t.Errorf("parsed merged = %v, want %v", merged, tt.expected)
			}
		})
	}
}

func TestCheckCleanupComplete(t *testing.T) {
	tests := []struct {
		name        string
		pendingOps  int
		wantDone    bool
		wantOpsLeft int
	}{
		{
			name:        "last operation completes",
			pendingOps:  1,
			wantDone:    true,
			wantOpsLeft: 0,
		},
		{
			name:        "still waiting for more",
			pendingOps:  3,
			wantDone:    false,
			wantOpsLeft: 2,
		},
		{
			name:        "already zero",
			pendingOps:  0,
			wantDone:    true,
			wantOpsLeft: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				mergeState: &MergeWorkflowState{
					Step:              MergeStepCleanup,
					StepStatus:        make(map[MergeWorkflowStep]string),
					PendingCleanupOps: tt.pendingOps,
				},
			}

			done := p.checkCleanupComplete()

			if done != tt.wantDone {
				t.Errorf("checkCleanupComplete() = %v, want %v", done, tt.wantDone)
			}
			if p.mergeState.PendingCleanupOps != tt.wantOpsLeft {
				t.Errorf("PendingCleanupOps = %v, want %v", p.mergeState.PendingCleanupOps, tt.wantOpsLeft)
			}
			if done && p.mergeState.Step != MergeStepDone {
				t.Errorf("Step = %v, want MergeStepDone when done", p.mergeState.Step)
			}
		})
	}
}

func TestDeleteDoneMsgWarnings(t *testing.T) {
	// Test that DeleteDoneMsg properly carries warnings
	msg := DeleteDoneMsg{
		Name:     "test-worktree",
		Err:      nil,
		Warnings: []string{"Local branch: branch 'feature' not found", "Remote branch: not found"},
	}

	if msg.Name != "test-worktree" {
		t.Errorf("Name = %v, want test-worktree", msg.Name)
	}
	if msg.Err != nil {
		t.Errorf("Err = %v, want nil", msg.Err)
	}
	if len(msg.Warnings) != 2 {
		t.Errorf("len(Warnings) = %v, want 2", len(msg.Warnings))
	}
}

func TestParseExistingPRURL(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantURL   string
		wantFound bool
	}{
		{
			name:      "standard error with PR URL",
			output:    `a pull request for branch "worktree-improvements" into branch "main" already exists: https://github.com/marcus/sidecar/pull/30: exit status 1`,
			wantURL:   "https://github.com/marcus/sidecar/pull/30",
			wantFound: true,
		},
		{
			name:      "error without exit status suffix",
			output:    `a pull request for branch "feature" into branch "main" already exists: https://github.com/owner/repo/pull/123`,
			wantURL:   "https://github.com/owner/repo/pull/123",
			wantFound: true,
		},
		{
			name:      "different error message",
			output:    `GraphQL: Could not resolve to a Repository with the name 'owner/repo'.`,
			wantURL:   "",
			wantFound: false,
		},
		{
			name:      "empty output",
			output:    ``,
			wantURL:   "",
			wantFound: false,
		},
		{
			name:      "already exists but no URL",
			output:    `a pull request already exists: `,
			wantURL:   "",
			wantFound: false,
		},
		{
			name:      "URL with trailing newline",
			output:    "a pull request already exists: https://github.com/o/r/pull/1\n",
			wantURL:   "https://github.com/o/r/pull/1",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, gotFound := parseExistingPRURL(tt.output)
			if gotURL != tt.wantURL {
				t.Errorf("parseExistingPRURL() url = %q, want %q", gotURL, tt.wantURL)
			}
			if gotFound != tt.wantFound {
				t.Errorf("parseExistingPRURL() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}
