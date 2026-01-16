package app

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeInfo contains information about a git worktree.
type WorktreeInfo struct {
	Path   string // Absolute path to the worktree
	Branch string // Branch name (e.g., "feature-auth")
	IsMain bool   // True if this is the main worktree
}

// GetWorktrees returns all worktrees for the repository containing workDir.
// Returns nil if workDir is not in a git repository.
func GetWorktrees(workDir string) []WorktreeInfo {
	// First, verify this is a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return nil
	}

	// Get list of worktrees
	cmd = exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseWorktreeList(string(output))
}

// parseWorktreeList parses the porcelain output of `git worktree list`.
// Format is:
//
//	worktree /path/to/worktree
//	HEAD <sha>
//	branch refs/heads/branch-name
//	<blank line>
func parseWorktreeList(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var current WorktreeInfo
	isFirst := true

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				current.IsMain = isFirst
				worktrees = append(worktrees, current)
				isFirst = false
			}
			current = WorktreeInfo{}
			continue
		}

		if path, found := strings.CutPrefix(line, "worktree "); found {
			current.Path = filepath.Clean(path)
		} else if branchRef, found := strings.CutPrefix(line, "branch "); found {
			// Extract branch name from refs/heads/branch-name
			current.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
		}
		// We ignore HEAD and other lines
	}

	// Handle last entry if no trailing newline
	if current.Path != "" {
		current.IsMain = isFirst
		worktrees = append(worktrees, current)
	}

	return worktrees
}

// GetMainWorktreePath returns the path to the main worktree for the repository.
// Returns empty string if not in a git repo or no main worktree found.
func GetMainWorktreePath(workDir string) string {
	worktrees := GetWorktrees(workDir)
	for _, wt := range worktrees {
		if wt.IsMain {
			return wt.Path
		}
	}
	return ""
}

// GetWorktreeName returns the worktree name for display (branch name or directory name).
// Returns empty string if workDir is not in a worktree or is the main worktree.
func GetWorktreeName(workDir string) string {
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		return ""
	}
	cleanPath := filepath.Clean(absPath)

	worktrees := GetWorktrees(workDir)
	for _, wt := range worktrees {
		if wt.Path == cleanPath && !wt.IsMain {
			if wt.Branch != "" {
				return wt.Branch
			}
			return filepath.Base(wt.Path)
		}
	}
	return ""
}

// GetAllRelatedPaths returns all paths that share the same git repository:
// the main worktree and all linked worktrees. Each path is absolute.
// Returns nil if workDir is not in a git repository.
func GetAllRelatedPaths(workDir string) []string {
	worktrees := GetWorktrees(workDir)
	if len(worktrees) == 0 {
		return nil
	}

	paths := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		paths = append(paths, wt.Path)
	}
	return paths
}

// IsWorktree returns true if workDir is a linked worktree (not the main worktree).
func IsWorktree(workDir string) bool {
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		return false
	}
	cleanPath := filepath.Clean(absPath)

	worktrees := GetWorktrees(workDir)
	for _, wt := range worktrees {
		if wt.Path == cleanPath {
			return !wt.IsMain
		}
	}
	return false
}

// WorktreeNameForPath returns the worktree name for a given absolute path.
// Returns empty string if the path is the main worktree or not found.
func WorktreeNameForPath(workDir, targetPath string) string {
	cleanTarget := filepath.Clean(targetPath)
	worktrees := GetWorktrees(workDir)
	for _, wt := range worktrees {
		if wt.Path == cleanTarget && !wt.IsMain {
			if wt.Branch != "" {
				return wt.Branch
			}
			return filepath.Base(wt.Path)
		}
	}
	return ""
}

// GetRepoName returns the git repository name for the given directory.
// It tries to extract the name from the remote URL first, falling back
// to the directory name if no remote is configured.
// Returns empty string if the directory is not a git repository.
func GetRepoName(workDir string) string {
	// Check if this is a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return ""
	}

	// Try to get remote URL
	cmd = exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		if name := parseRepoNameFromURL(url); name != "" {
			return name
		}
	}

	// Fallback to directory name
	absPath, err := filepath.Abs(workDir)
	if err != nil {
		return ""
	}
	return filepath.Base(absPath)
}

// parseRepoNameFromURL extracts the repository name from a git URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS URLs.
func parseRepoNameFromURL(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH format: git@github.com:user/repo
	if idx := strings.LastIndex(url, ":"); idx != -1 && !strings.Contains(url, "://") {
		url = url[idx+1:]
	}

	// Handle HTTPS format: https://github.com/user/repo
	if idx := strings.LastIndex(url, "/"); idx != -1 {
		return url[idx+1:]
	}

	return url
}
