package worktree

import (
	"os/exec"
	"path/filepath"
	"strings"

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
	return func() tea.Msg {
		content, raw, err := getDiff(path)
		if err != nil {
			return DiffErrorMsg{WorktreeName: name, Err: err}
		}
		return DiffLoadedMsg{WorktreeName: name, Content: content, Raw: raw}
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
		baseBranch = "main"
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

