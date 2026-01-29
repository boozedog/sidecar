package app

import (
	"encoding/json"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// IssueSearchResult holds a single search result from td search.
type IssueSearchResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
}

// tdSearchResultWrapper wraps td search JSON output: {"Issue": {...}, "Score": N}.
type tdSearchResultWrapper struct {
	Issue IssueSearchResult `json:"Issue"`
	Score int               `json:"Score"`
}

// IssueSearchResultMsg carries search results back to the app.
type IssueSearchResultMsg struct {
	Query   string
	Results []IssueSearchResult
	Error   error
}

// issueSearchCmd runs `td search <query> --json -n 10` asynchronously.
func issueSearchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("td", "search", query, "--json", "-n", "10").Output()
		if err != nil {
			return IssueSearchResultMsg{Query: query, Error: err}
		}
		var wrappers []tdSearchResultWrapper
		if err := json.Unmarshal(out, &wrappers); err != nil {
			return IssueSearchResultMsg{Query: query, Error: err}
		}
		results := make([]IssueSearchResult, len(wrappers))
		for i, w := range wrappers {
			results[i] = w.Issue
		}
		return IssueSearchResultMsg{Query: query, Results: results}
	}
}

// IssuePreviewData holds lightweight issue data fetched via CLI.
type IssuePreviewData struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Type        string   `json:"type"`
	Priority    string   `json:"priority"`
	Points      int      `json:"points"`
	Description string   `json:"description"`
	ParentID    string   `json:"parent_id"`
	Labels      []string `json:"labels"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// IssuePreviewResultMsg carries fetched issue data back to the app.
type IssuePreviewResultMsg struct {
	Data  *IssuePreviewData
	Error error
}

// OpenFullIssueMsg is broadcast to plugins to open the full rich issue view.
// Currently handled by the TD monitor plugin via monitor.OpenIssueByIDMsg.
type OpenFullIssueMsg struct {
	IssueID string
}

// fetchIssuePreviewCmd runs `td show <id> -f json` and returns the result.
func fetchIssuePreviewCmd(issueID string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("td", "show", issueID, "-f", "json").Output()
		if err != nil {
			return IssuePreviewResultMsg{Error: err}
		}
		var data IssuePreviewData
		if err := json.Unmarshal(out, &data); err != nil {
			return IssuePreviewResultMsg{Error: err}
		}
		return IssuePreviewResultMsg{Data: &data}
	}
}
