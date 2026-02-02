package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcus/sidecar/internal/app"
)

// fetchPRList runs gh pr list and returns open PRs.
func (p *Plugin) fetchPRList() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		cmd := exec.Command("gh", "pr", "list",
			"--json", "number,title,headRefName,url,isDraft,createdAt,author",
			"--limit", "30",
		)
		cmd.Dir = workDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return FetchPRListMsg{Err: fmt.Errorf("gh pr list: %s", strings.TrimSpace(stderr.String()))}
		}

		var prs []PRListItem
		if err := json.Unmarshal(output, &prs); err != nil {
			return FetchPRListMsg{Err: fmt.Errorf("parse pr list: %w", err)}
		}

		return FetchPRListMsg{PRs: prs}
	}
}

// fetchAndCreateWorktree fetches a PR branch and creates a worktree from it.
func (p *Plugin) fetchAndCreateWorktree(pr PRListItem) tea.Cmd {
	workDir := p.ctx.WorkDir
	dirPrefix := p.ctx.Config != nil && p.ctx.Config.Plugins.Workspace.DirPrefix

	return func() tea.Msg {
		branch := pr.Branch

		// Fetch the remote branch
		fetchCmd := exec.Command("git", "fetch", "origin", branch)
		fetchCmd.Dir = workDir
		if output, err := fetchCmd.CombinedOutput(); err != nil {
			return FetchPRDoneMsg{Err: fmt.Errorf("git fetch: %s", strings.TrimSpace(string(output)))}
		}

		// Determine worktree path
		dirName := branch
		if dirPrefix {
			repoName := app.GetRepoName(workDir)
			if repoName != "" {
				dirName = repoName + "-" + branch
			}
		}
		parentDir := filepath.Dir(workDir)
		wtPath := filepath.Join(parentDir, dirName)

		// Create worktree tracking the remote branch
		addCmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "origin/"+branch)
		addCmd.Dir = workDir
		if output, err := addCmd.CombinedOutput(); err != nil {
			outStr := strings.TrimSpace(string(output))
			// Check if branch already exists locally
			if strings.Contains(outStr, "already exists") {
				return FetchPRDoneMsg{Err: fmt.Errorf("branch %q already exists locally", branch)}
			}
			return FetchPRDoneMsg{Err: fmt.Errorf("git worktree add: %s", outStr)}
		}

		// Write .sidecar-pr file with PR URL
		if err := savePRURL(wtPath, pr.URL); err != nil {
			// Non-fatal
		}

		// Detect base branch for diff
		baseBranch := detectDefaultBranch(workDir)

		wt := &Worktree{
			Name:       dirName,
			Path:       wtPath,
			Branch:     branch,
			BaseBranch: baseBranch,
			PRURL:      pr.URL,
			Status:     StatusPaused,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		return FetchPRDoneMsg{Worktree: wt}
	}
}

// filteredFetchPRItems returns PR items matching the current filter.
func (p *Plugin) filteredFetchPRItems() []PRListItem {
	if p.fetchPRFilter == "" {
		return p.fetchPRItems
	}
	query := strings.ToLower(p.fetchPRFilter)
	var matches []PRListItem
	for _, pr := range p.fetchPRItems {
		if strings.Contains(strings.ToLower(pr.Title), query) ||
			strings.Contains(strings.ToLower(pr.Branch), query) ||
			strings.Contains(strings.ToLower(pr.Author.Login), query) ||
			strings.Contains(fmt.Sprintf("#%d", pr.Number), query) {
			matches = append(matches, pr)
		}
	}
	return matches
}

// adjustFetchPRScroll keeps the cursor visible within the 10-item window.
func (p *Plugin) adjustFetchPRScroll() {
	const maxVisible = 10
	if p.fetchPRCursor < p.fetchPRScrollOffset {
		p.fetchPRScrollOffset = p.fetchPRCursor
	}
	if p.fetchPRCursor >= p.fetchPRScrollOffset+maxVisible {
		p.fetchPRScrollOffset = p.fetchPRCursor - maxVisible + 1
	}
}

// clearFetchPRState resets fetch PR modal state.
func (p *Plugin) clearFetchPRState() {
	p.fetchPRItems = nil
	p.fetchPRFilter = ""
	p.fetchPRCursor = 0
	p.fetchPRScrollOffset = 0
	p.fetchPRLoading = false
	p.fetchPRError = ""
	p.clearFetchPRModal()
}
