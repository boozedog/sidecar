package workspace

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// loadStats returns a command to load git stats for a worktree.
func (p *Plugin) loadStats(path string) tea.Cmd {
	epoch := p.ctx.Epoch // Capture epoch for stale detection
	return func() tea.Msg {
		name := filepath.Base(path)
		stats, err := computeStats(path)
		if err != nil {
			return StatsErrorMsg{WorkspaceName: name, Err: err}
		}
		return StatsLoadedMsg{Epoch: epoch, WorkspaceName: name, Stats: stats}
	}
}

// computeStats calculates git stats for a worktree.
func computeStats(workdir string) (*GitStats, error) {
	stats := &GitStats{}

	// Get diff stats (uncommitted changes)
	if err := getDiffStats(workdir, stats); err != nil {
		return nil, err
	}

	// Get ahead/behind counts
	if err := getAheadBehind(workdir, stats); err != nil {
		// Non-fatal: might not have upstream
	}

	return stats, nil
}

// getDiffStats computes additions/deletions from git diff.
func getDiffStats(workdir string, stats *GitStats) error {
	// Get stats for both staged and unstaged changes
	cmd := exec.Command("git", "diff", "--stat", "HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		// No HEAD yet or other error, try without HEAD
		cmd = exec.Command("git", "diff", "--stat")
		cmd.Dir = workdir
		output, _ = cmd.Output()
	}

	// Parse stat output: " 3 files changed, 10 insertions(+), 5 deletions(-)"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
			// Parse insertions
			if idx := strings.Index(line, "insertion"); idx > 0 {
				parts := strings.Split(line[:idx], " ")
				for i := len(parts) - 1; i >= 0; i-- {
					if n, err := strconv.Atoi(parts[i]); err == nil {
						stats.Additions = n
						break
					}
				}
			}
			// Parse deletions
			if idx := strings.Index(line, "deletion"); idx > 0 {
				parts := strings.Split(line[:idx], " ")
				for i := len(parts) - 1; i >= 0; i-- {
					if n, err := strconv.Atoi(parts[i]); err == nil {
						stats.Deletions = n
						break
					}
				}
			}
			// Parse files changed
			if idx := strings.Index(line, "file"); idx > 0 {
				parts := strings.Split(line[:idx], " ")
				for i := len(parts) - 1; i >= 0; i-- {
					if n, err := strconv.Atoi(strings.TrimSpace(parts[i])); err == nil {
						stats.FilesChanged = n
						break
					}
				}
			}
		}
	}

	return nil
}

// getAheadBehind computes ahead/behind counts from upstream.
func getAheadBehind(workdir string, stats *GitStats) error {
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	cmd.Dir = workdir
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) == 2 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			stats.Behind = n
		}
		if n, err := strconv.Atoi(parts[1]); err == nil {
			stats.Ahead = n
		}
	}

	return nil
}
