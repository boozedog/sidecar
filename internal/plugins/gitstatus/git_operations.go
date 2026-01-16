package gitstatus

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// doCommit executes the git commit asynchronously.
func (p *Plugin) doCommit(message string) tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		hash, err := ExecuteCommit(workDir, message)
		if err != nil {
			return CommitErrorMsg{Err: err}
		}
		// Extract first line as subject
		subject := strings.Split(message, "\n")[0]
		return CommitSuccessMsg{Hash: hash, Subject: subject}
	}
}

// doPush executes a git push asynchronously.
func (p *Plugin) doPush(force bool) tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		output, err := ExecutePush(workDir, force)
		if err != nil {
			return PushErrorMsg{Err: err}
		}
		return PushSuccessMsg{Output: output}
	}
}

// doPushForce executes a force push with lease.
func (p *Plugin) doPushForce() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		output, err := ExecutePushForce(workDir)
		if err != nil {
			return PushErrorMsg{Err: err}
		}
		return PushSuccessMsg{Output: output}
	}
}

// doPushSetUpstream executes a push with upstream tracking.
func (p *Plugin) doPushSetUpstream() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		output, err := ExecutePushSetUpstream(workDir)
		if err != nil {
			return PushErrorMsg{Err: err}
		}
		return PushSuccessMsg{Output: output}
	}
}

// canPush returns true if there are commits that can be pushed.
func (p *Plugin) canPush() bool {
	return p.pushStatus != nil && p.pushStatus.CanPush()
}

// doStashPush stashes all current changes.
func (p *Plugin) doStashPush() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		err := StashPush(workDir)
		return StashResultMsg{Operation: "push", Err: err}
	}
}

// doStashPop pops the latest stash.
func (p *Plugin) doStashPop() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		err := StashPop(workDir)
		return StashResultMsg{Operation: "pop", Ref: "stash@{0}", Err: err}
	}
}


// doFetch fetches from remote.
func (p *Plugin) doFetch() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		output, err := ExecuteFetch(workDir)
		if err != nil {
			return FetchErrorMsg{Err: err}
		}
		return FetchSuccessMsg{Output: output}
	}
}

// doPull pulls from remote.
func (p *Plugin) doPull() tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		output, err := ExecutePull(workDir)
		if err != nil {
			return PullErrorMsg{Err: err}
		}
		return PullSuccessMsg{Output: output}
	}
}

// doDiscard executes the git discard operation.
func (p *Plugin) doDiscard(entry *FileEntry) tea.Cmd {
	workDir := p.ctx.WorkDir
	return func() tea.Msg {
		var err error
		if entry.Status == StatusUntracked {
			// Remove untracked file
			err = DiscardUntracked(workDir, entry.Path)
		} else if entry.Staged {
			// Unstage and restore staged file
			err = DiscardStaged(workDir, entry.Path)
		} else {
			// Restore modified file
			err = DiscardModified(workDir, entry.Path)
		}
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return RefreshDoneMsg{}
	}
}
