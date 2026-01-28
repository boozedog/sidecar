package workspace

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// loadSelectedDiff returns a command to load diff for the selected worktree.
// Also loads task details if Task tab is active.
func (p *Plugin) loadSelectedDiff() tea.Cmd {
	wt := p.selectedWorktree()
	if wt == nil {
		return nil
	}

	cmds := []tea.Cmd{p.loadDiff(wt.Path, wt.Name)}

	// Also load task details if Task tab is active
	if p.previewTab == PreviewTabTask && wt.TaskID != "" {
		cmds = append(cmds, p.loadTaskDetailsIfNeeded())
	}

	return tea.Batch(cmds...)
}

// loadDiff returns a command to load diff for a worktree.
func (p *Plugin) loadDiff(path, name string) tea.Cmd {
	epoch := p.ctx.Epoch // Capture epoch for stale detection
	return func() tea.Msg {
		content, raw, err := getDiff(path)
		if err != nil {
			return DiffErrorMsg{WorkspaceName: name, Err: err}
		}
		return DiffLoadedMsg{Epoch: epoch, WorkspaceName: name, Content: content, Raw: raw}
	}
}

// getDiff returns the diff for a worktree.
func getDiff(workdir string) (content, raw string, err error) {
	// Get combined staged and unstaged diff
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		// No HEAD yet, try just staged/unstaged
		cmd = exec.Command("git", "diff")
		cmd.Dir = workdir
		output, _ = cmd.Output()
	}

	raw = string(output)

	// For now, content is same as raw
	// Later can add syntax highlighting or delta processing
	content = raw

	return content, raw, nil
}

// getDiffFromBase returns diff compared to base branch.
func getDiffFromBase(workdir, baseBranch string) (string, error) {
	if baseBranch == "" {
		baseBranch = detectDefaultBranch(workdir)
	}

	// Try to find merge-base first
	mbCmd := exec.Command("git", "merge-base", baseBranch, "HEAD")
	mbCmd.Dir = workdir
	mbOutput, err := mbCmd.Output()

	var args []string
	if err == nil {
		// Use merge-base for cleaner diff
		// Trim whitespace and validate length before slicing
		mbHash := strings.TrimSpace(string(mbOutput))
		if len(mbHash) >= 40 {
			args = []string{"diff", mbHash[:40] + "..HEAD"}
		} else {
			// Invalid merge-base output, fall back
			args = []string{"diff", baseBranch + "..HEAD"}
		}
	} else {
		// Fall back to direct comparison
		args = []string{"diff", baseBranch + "..HEAD"}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// getDiffStatFromBase returns the --stat output compared to the base branch.
func getDiffStatFromBase(workdir, baseBranch string) (string, error) {
	if baseBranch == "" {
		baseBranch = detectDefaultBranch(workdir)
	}

	// Try to find merge-base first
	mbCmd := exec.Command("git", "merge-base", baseBranch, "HEAD")
	mbCmd.Dir = workdir
	mbOutput, err := mbCmd.Output()

	var args []string
	if err == nil {
		mbHash := strings.TrimSpace(string(mbOutput))
		if len(mbHash) >= 40 {
			args = []string{"diff", "--stat", mbHash[:40] + "..HEAD"}
		} else {
			args = []string{"diff", "--stat", baseBranch + "..HEAD"}
		}
	} else {
		args = []string{"diff", "--stat", baseBranch + "..HEAD"}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// getDiffSummary returns a brief summary of changes.
func getDiffSummary(workdir string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", "HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// getFilesChanged returns the list of changed files.
func getFilesChanged(workdir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	name := filepath.Base(workdir)
	for _, line := range splitLines(string(output)) {
		if line != "" {
			files = append(files, name+"/"+line)
		}
	}
	return files, nil
}

// splitLines splits a string into lines, handling various line endings.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// loadCommitStatus returns a command to load commit status for a worktree.
func (p *Plugin) loadCommitStatus(wt *Worktree) tea.Cmd {
	if wt == nil {
		return nil
	}
	epoch := p.ctx.Epoch // Capture epoch for stale detection
	name := wt.Name
	path := wt.Path
	baseBranch := wt.BaseBranch

	return func() tea.Msg {
		commits, err := getWorktreeCommits(path, baseBranch)
		if err != nil {
			return CommitStatusLoadedMsg{Epoch: epoch, WorkspaceName: name, Err: err}
		}
		return CommitStatusLoadedMsg{Epoch: epoch, WorkspaceName: name, Commits: commits}
	}
}

// getWorktreeCommits returns commits unique to this branch vs base branch with status.
func getWorktreeCommits(workdir, baseBranch string) ([]CommitStatusInfo, error) {
	// If baseBranch is empty, detect the default branch
	if baseBranch == "" {
		baseBranch = detectDefaultBranch(workdir)
	}

	// Try to get commits comparing against base branch
	output, err := tryGitLog(workdir, baseBranch)
	if err != nil {
		// Try origin/baseBranch
		output, err = tryGitLog(workdir, "origin/"+baseBranch)
	}
	if err != nil {
		// Last resort: detect default branch fresh (in case baseBranch was stale/wrong)
		detected := detectDefaultBranch(workdir)
		if detected != baseBranch {
			output, err = tryGitLog(workdir, detected)
			if err != nil {
				output, err = tryGitLog(workdir, "origin/"+detected)
			}
			baseBranch = detected // Update for merged check below
		}
	}
	if err != nil {
		// No commits or error - return empty list
		return []CommitStatusInfo{}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return []CommitStatusInfo{}, nil
	}

	// Get remote tracking branch to check pushed status
	remoteBranch := getRemoteTrackingBranch(workdir)

	var commits []CommitStatusInfo
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		subject := parts[1]

		// Check if pushed (exists in remote tracking branch)
		pushed := false
		if remoteBranch != "" {
			pushed = isCommitInBranch(workdir, hash, remoteBranch)
		}

		// Check if merged (exists in base branch) - should always be false for these commits
		// but include for potential future merge detection
		merged := isCommitInBranch(workdir, hash, baseBranch)

		commits = append(commits, CommitStatusInfo{
			Hash:    hash,
			Subject: subject,
			Pushed:  pushed,
			Merged:  merged,
		})
	}

	return commits, nil
}

// tryGitLog attempts to get commit log comparing HEAD to a base ref.
func tryGitLog(workdir, baseRef string) ([]byte, error) {
	cmd := exec.Command("git", "log", baseRef+"..HEAD", "--oneline", "--format=%h|%s")
	cmd.Dir = workdir
	return cmd.Output()
}

// detectDefaultBranch detects the default branch for a repository.
// Checks remote HEAD first, then falls back to common names.
func detectDefaultBranch(workdir string) string {
	// Try to get the remote HEAD (most reliable)
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		if branch, found := strings.CutPrefix(ref, "refs/remotes/origin/"); found {
			return branch
		}
	}

	// Fallback: check which common branch exists
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		cmd.Dir = workdir
		if err := cmd.Run(); err == nil {
			return branch
		}
	}

	// Last resort default
	return "main"
}

// resolveBaseBranch returns the worktree's BaseBranch if set,
// otherwise detects the default branch from the worktree's repo.
func resolveBaseBranch(wt *Worktree) string {
	if wt.BaseBranch != "" {
		return wt.BaseBranch
	}
	return detectDefaultBranch(wt.Path)
}

// getRemoteTrackingBranch returns the remote tracking branch for HEAD.
func getRemoteTrackingBranch(workdir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// isCommitInBranch checks if a commit is reachable from a branch.
// Uses git merge-base --is-ancestor with a 5-second timeout.
// Returns false for empty inputs, non-existent refs, or if commit is not an ancestor.
func isCommitInBranch(workdir, commit, branch string) bool {
	if commit == "" || branch == "" || workdir == "" {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", commit, branch)
	cmd.Dir = workdir
	err := cmd.Run()
	return err == nil
}

