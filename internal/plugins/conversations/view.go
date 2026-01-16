package conversations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/marcus/sidecar/internal/adapter"
	"github.com/marcus/sidecar/internal/styles"
	"github.com/marcus/sidecar/internal/ui"
)

// ansiBackgroundRegex matches ANSI background color escape sequences including:
// - Basic: \x1b[40m through \x1b[49m
// - 256-color: \x1b[48;5;XXXm
// - True color: \x1b[48;2;R;G;Bm
var ansiBackgroundRegex = regexp.MustCompile(`\x1b\[(4[0-9]|48;[0-9;]+)m`)

// stripANSIBackground removes ANSI background color codes from a string
// to allow selection highlighting to show through consistently.
func stripANSIBackground(s string) string {
	return ansiBackgroundRegex.ReplaceAllString(s, "")
}

// renderNoAdapter renders the view when no adapter is available.
func renderNoAdapter() string {
	return styles.Muted.Render(" No AI sessions available")
}

// getSessionGroup returns the time group label for a given timestamp.
func getSessionGroup(t time.Time) string {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)

	switch {
	case t.After(today) || t.Equal(today):
		return "Today"
	case t.After(yesterday) || t.Equal(yesterday):
		return "Yesterday"
	case t.After(weekAgo):
		return "This Week"
	default:
		return "Older"
	}
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}

// formatTokens formats token counts compactly.
func formatTokens(input, output, cache int) string {
	parts := []string{}

	if input > 0 {
		parts = append(parts, fmt.Sprintf("in:%s", formatK(input)))
	}
	if output > 0 {
		parts = append(parts, fmt.Sprintf("out:%s", formatK(output)))
	}
	if cache > 0 {
		parts = append(parts, fmt.Sprintf("$:%s", formatK(cache)))
	}

	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " ") + ")"
}

// formatK formats a number with K/M suffix.
func formatK(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatCost formats a cost estimate in dollars.
func formatCost(cost float64) string {
	if cost < 0.01 {
		return "<$0.01"
	}
	if cost < 1.0 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.1f", cost)
}

func adapterBreakdown(sessions []adapter.Session) string {
	counts := make(map[string]int)
	for _, session := range sessions {
		icon := session.AdapterIcon
		if icon == "" {
			icon = adapterAbbrev(session)
		}
		if icon == "" {
			continue
		}
		counts[icon]++
	}
	if len(counts) <= 1 {
		return ""
	}
	var keys []string
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d %s", counts[key], key))
	}
	return strings.Join(parts, ", ")
}

func adapterBadgeText(session adapter.Session) string {
	if session.AdapterIcon != "" {
		return session.AdapterIcon
	}
	// Fallback for sessions without icon
	abbr := adapterAbbrev(session)
	if abbr == "" {
		return "?" // Unknown adapter fallback
	}
	return "●" + abbr
}

// renderAdapterIcon returns a colorized adapter icon based on the adapter type.
func renderAdapterIcon(session adapter.Session) string {
	icon := session.AdapterIcon
	if icon == "" {
		icon = "◆"
	}

	// Color based on adapter
	switch session.AdapterID {
	case "claude-code":
		// Amber for Claude Code (matches existing StatusModified)
		return styles.StatusModified.Render(icon)
	case "gemini-cli":
		// Google blue
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")).Render(icon)
	case "codex":
		// OpenAI green
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#10A37F")).Render(icon)
	case "cursor-cli":
		// Cursor purple
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Render(icon)
	default:
		return styles.Muted.Render(icon)
	}
}

func adapterAbbrev(session adapter.Session) string {
	switch session.AdapterID {
	case "claude-code":
		return "CC"
	case "codex":
		return "CX"
	case "opencode":
		return "OC"
	case "gemini-cli":
		return "GC"
	default:
		name := session.AdapterName
		if name == "" {
			name = session.AdapterID
		}
		name = strings.ReplaceAll(name, " ", "")
		if name == "" {
			return ""
		}
		if runes := []rune(name); len(runes) <= 2 {
			return strings.ToUpper(name)
		} else {
			return strings.ToUpper(string(runes[:2]))
		}
	}
}

func adapterShortName(session *adapter.Session) string {
	if session == nil {
		return ""
	}
	switch session.AdapterID {
	case "claude-code":
		return "claude"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	case "gemini-cli":
		return "gemini"
	default:
		if session.AdapterName != "" {
			return strings.ToLower(session.AdapterName)
		}
		return session.AdapterID
	}
}

type adapterFilterOption struct {
	key  string
	id   string
	name string
}

func adapterFilterOptions(adapters map[string]adapter.Adapter) []adapterFilterOption {
	if len(adapters) == 0 {
		return nil
	}

	reservedKeys := map[string]bool{
		"1": true,
		"2": true,
		"3": true,
		"t": true,
		"y": true,
		"w": true,
		"a": true,
		"x": true,
	}

	usedKeys := make(map[string]bool)
	var options []adapterFilterOption

	addOption := func(id string, name string, key string) {
		if key == "" || usedKeys[key] || reservedKeys[key] {
			return
		}
		usedKeys[key] = true
		options = append(options, adapterFilterOption{key: key, id: id, name: name})
	}

	if a, ok := adapters["claude-code"]; ok {
		addOption("claude-code", a.Name(), "c")
	}
	if a, ok := adapters["codex"]; ok {
		addOption("codex", a.Name(), "o")
	}
	if a, ok := adapters["opencode"]; ok {
		addOption("opencode", a.Name(), "p")
	}
	if a, ok := adapters["gemini-cli"]; ok {
		addOption("gemini-cli", a.Name(), "g")
	}

	var extra []adapterFilterOption
	for id, a := range adapters {
		if id == "claude-code" || id == "codex" || id == "opencode" || id == "gemini-cli" {
			continue
		}
		name := a.Name()
		if name == "" {
			name = id
		}
		key := ""
		for _, r := range strings.ToLower(name) {
			candidate := string(r)
			if usedKeys[candidate] || reservedKeys[candidate] {
				continue
			}
			key = candidate
			break
		}
		if key != "" {
			usedKeys[key] = true
			extra = append(extra, adapterFilterOption{key: key, id: id, name: name})
		}
	}

	sort.Slice(extra, func(i, j int) bool {
		return extra[i].name < extra[j].name
	})

	options = append(options, extra...)
	return options
}

func resumeCommand(session *adapter.Session) string {
	if session == nil || session.ID == "" {
		return ""
	}
	switch session.AdapterID {
	case "claude-code":
		return fmt.Sprintf("claude --resume %s", session.ID)
	case "codex":
		return fmt.Sprintf("codex resume %s", session.ID)
	case "opencode":
		return fmt.Sprintf("opencode --continue -s %s", session.ID)
	case "gemini-cli":
		return fmt.Sprintf("gemini --resume %s", session.ID)
	case "cursor-cli":
		return fmt.Sprintf("cursor-agent --resume %s", session.ID)
	default:
		return ""
	}
}

// modelShortName maps model IDs to short display names.
func modelShortName(model string) string {
	model = strings.ToLower(model)
	switch {
	// Claude models (cursor uses "claude-4.5-opus-high-thinking" etc.)
	case strings.Contains(model, "opus"):
		return "opus"
	case strings.Contains(model, "sonnet-4") || strings.Contains(model, "sonnet4"):
		return "sonnet4"
	case strings.Contains(model, "sonnet"):
		return "sonnet"
	case strings.Contains(model, "haiku"):
		return "haiku"
	// GPT models
	case strings.HasPrefix(model, "gpt-"):
		parts := strings.Split(model, "-")
		if len(parts) > 1 {
			return "gpt" + parts[1]
		}
		return "gpt"
	case strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3"):
		parts := strings.Split(model, "-")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
		return "o"
	// Gemini models
	case strings.Contains(model, "gemini-3-pro") || strings.Contains(model, "gemini3-pro"):
		return "3Pro"
	case strings.Contains(model, "gemini-3-flash") || strings.Contains(model, "gemini3-flash"):
		return "3Flash"
	case strings.Contains(model, "gemini-3") || strings.Contains(model, "gemini3"):
		return "gemini3"
	case strings.Contains(model, "gemini-2.0-flash"):
		return "2Flash"
	case strings.Contains(model, "gemini-1.5-pro"):
		return "1.5Pro"
	case strings.Contains(model, "gemini-1.5-flash"):
		return "1.5Flash"
	case strings.HasPrefix(model, "gemini"):
		return "gemini"
	// Other models
	case strings.HasPrefix(model, "grok"):
		return "grok"
	case strings.HasPrefix(model, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(model, "mistral"):
		return "mistral"
	case strings.HasPrefix(model, "llama"):
		return "llama"
	case strings.HasPrefix(model, "qwen"):
		return "qwen"
	default:
		return ""
	}
}

// renderModelBadge returns a colorful styled badge for the model name.
// opus=purple, sonnet=green, haiku=blue, others=default code style.
func renderModelBadge(model string) string {
	short := modelShortName(model)
	if short == "" {
		return ""
	}

	var badgeStyle lipgloss.Style
	switch short {
	case "opus":
		// Purple/magenta for opus (premium model)
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#C084FC")).
			Background(lipgloss.Color("#3B1F5B")).
			Padding(0, 1)
	case "sonnet", "sonnet4":
		// Green for sonnet (balanced)
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#86EFAC")).
			Background(lipgloss.Color("#14532D")).
			Padding(0, 1)
	case "haiku":
		// Blue for haiku (fast/cheap)
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93C5FD")).
			Background(lipgloss.Color("#1E3A5F")).
			Padding(0, 1)
	case "gpt4", "gpt4o":
		// Teal for GPT-4
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5EEAD4")).
			Background(lipgloss.Color("#134E4A")).
			Padding(0, 1)
	case "o1", "o3":
		// Orange for reasoning models
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FDBA74")).
			Background(lipgloss.Color("#7C2D12")).
			Padding(0, 1)
	case "gemini", "gemini3", "3Pro", "3Flash", "2Flash", "1.5Pro", "1.5Flash":
		// Google blue for Gemini
		badgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93C5FD")).
			Background(lipgloss.Color("#1E3A8A")).
			Padding(0, 1)
	default:
		// Default: amber/code style
		badgeStyle = lipgloss.NewStyle().
			Foreground(styles.Accent).
			Padding(0, 1)
	}

	return badgeStyle.Render(short)
}

// renderTokenFlow returns a compact token flow indicator (in→out).
func renderTokenFlow(in, out int) string {
	if in == 0 && out == 0 {
		return ""
	}
	return styles.Muted.Render(fmt.Sprintf("%s→%s", formatK(in), formatK(out)))
}

// formatSessionDuration formats session duration for display.
func formatSessionDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// estimateCost calculates cost in dollars based on model and tokens.
func estimateCost(model string, inputTokens, outputTokens, cacheRead int) float64 {
	var inRate, outRate float64
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "opus"):
		inRate, outRate = 15.0, 75.0
	case strings.Contains(model, "sonnet"):
		inRate, outRate = 3.0, 15.0
	case strings.Contains(model, "haiku"):
		inRate, outRate = 0.25, 1.25
	case strings.Contains(model, "gpt-4o"):
		inRate, outRate = 2.5, 10.0
	case strings.Contains(model, "gpt-4"):
		inRate, outRate = 10.0, 30.0
	case strings.Contains(model, "o1") || strings.Contains(model, "o3"):
		inRate, outRate = 15.0, 60.0
	case strings.Contains(model, "gemini"):
		inRate, outRate = 1.25, 5.0
	case strings.Contains(model, "deepseek"):
		inRate, outRate = 0.14, 0.28
	default:
		inRate, outRate = 3.0, 15.0 // Default to sonnet rates
	}

	// Cache reads get 90% discount
	regularIn := inputTokens - cacheRead
	if regularIn < 0 {
		regularIn = 0
	}
	cacheInCost := float64(cacheRead) * inRate * 0.1 / 1_000_000
	regularInCost := float64(regularIn) * inRate / 1_000_000
	outCost := float64(outputTokens) * outRate / 1_000_000

	return cacheInCost + regularInCost + outCost
}

// extractFilePath extracts file_path from tool input JSON.
func extractFilePath(input string) string {
	if input == "" {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return ""
	}
	if fp, ok := data["file_path"].(string); ok {
		return fp
	}
	return ""
}

// prettifyJSON attempts to format JSON output with indentation.
// Returns the original string if it's not valid JSON.
func prettifyJSON(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}

	// Check if it looks like JSON (starts with { or [)
	if s[0] != '{' && s[0] != '[' {
		return s
	}

	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return s
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return s
	}

	return string(pretty)
}

// extractToolCommand extracts a short command preview from tool input.
// Returns a truncated command string for display in tool headers.
func extractToolCommand(toolName, input string, maxLen int) string {
	if input == "" {
		return ""
	}

	// First try to parse as object
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err == nil {
		var cmd string
		switch toolName {
		case "Bash", "bash", "Shell", "shell":
			if c, ok := data["command"].(string); ok {
				cmd = c
			}
		case "Read", "read":
			if fp, ok := data["file_path"].(string); ok {
				return fp
			}
			if fp, ok := data["path"].(string); ok {
				return fp
			}
		case "Edit", "edit", "StrReplace", "str_replace_editor":
			if fp, ok := data["file_path"].(string); ok {
				return fp
			}
			if fp, ok := data["path"].(string); ok {
				return fp
			}
		case "Write", "write":
			if fp, ok := data["file_path"].(string); ok {
				return fp
			}
			if fp, ok := data["path"].(string); ok {
				return fp
			}
		case "Glob", "glob":
			if p, ok := data["pattern"].(string); ok {
				cmd = p
			}
			if p, ok := data["glob_pattern"].(string); ok {
				cmd = p
			}
		case "Grep", "grep":
			if p, ok := data["pattern"].(string); ok {
				cmd = p
			}
		case "Task", "task", "TodoWrite", "TodoRead":
			// Task tools often have text content
			if t, ok := data["text"].(string); ok {
				cmd = t
			}
			if t, ok := data["content"].(string); ok {
				cmd = t
			}
			if t, ok := data["description"].(string); ok {
				cmd = t
			}
		default:
			// Fallback: try common text fields
			if t, ok := data["text"].(string); ok {
				cmd = t
			} else if t, ok := data["content"].(string); ok {
				cmd = t
			} else if t, ok := data["message"].(string); ok {
				cmd = t
			} else if t, ok := data["query"].(string); ok {
				cmd = t
			}
		}

		if cmd != "" {
			// Clean up: remove newlines, collapse whitespace
			cmd = strings.ReplaceAll(cmd, "\n", " ")
			cmd = strings.Join(strings.Fields(cmd), " ")
			if len(cmd) > maxLen {
				cmd = cmd[:maxLen-3] + "..."
			}
			return cmd
		}
	}

	// Try to parse as array (e.g., [{"text": "..."}])
	var arr []map[string]any
	if err := json.Unmarshal([]byte(input), &arr); err == nil && len(arr) > 0 {
		// Extract text from first element
		if t, ok := arr[0]["text"].(string); ok {
			cmd := strings.ReplaceAll(t, "\n", " ")
			cmd = strings.Join(strings.Fields(cmd), " ")
			if len(cmd) > maxLen {
				cmd = cmd[:maxLen-3] + "..."
			}
			return cmd
		}
	}

	return ""
}

// renderTwoPane renders the two-pane layout with sessions on the left and messages on the right.
func (p *Plugin) renderTwoPane() string {
	// Check if hit regions need rebuilding (td-ea784b03)
	// Mark dirty if dimensions or scroll positions changed
	if p.width != p.prevWidth || p.height != p.prevHeight {
		p.hitRegionsDirty = true
		p.prevWidth = p.width
		p.prevHeight = p.height
	}
	if p.scrollOff != p.prevScrollOff {
		p.hitRegionsDirty = true
		p.prevScrollOff = p.scrollOff
	}
	if p.messageScroll != p.prevMsgScroll {
		p.hitRegionsDirty = true
		p.prevMsgScroll = p.messageScroll
	}
	if p.turnScrollOff != p.prevTurnScroll {
		p.hitRegionsDirty = true
		p.prevTurnScroll = p.turnScrollOff
	}

	// Pane height for panels (outer dimensions including borders)
	paneHeight := p.height
	if paneHeight < 4 {
		paneHeight = 4
	}

	// Inner content height (excluding borders and header lines)
	innerHeight := paneHeight - 2
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Handle collapsed sidebar - render full-width main pane
	if !p.sidebarVisible {
		mainWidth := p.width - 2 // Account for borders
		if mainWidth < 40 {
			mainWidth = 40
		}

		mainContent := p.renderMainPane(mainWidth, innerHeight)
		rightPane := styles.RenderPanel(mainContent, mainWidth, paneHeight, true)

		// Update hit regions for collapsed state
		if p.hitRegionsDirty {
			p.mouseHandler.HitMap.Clear()
			p.mouseHandler.HitMap.AddRect(regionMainPane, 0, 0, mainWidth, p.height, nil)
			p.registerTurnHitRegions(1, mainWidth-2, innerHeight)
			p.hitRegionsDirty = false
		}

		return rightPane
	}

	// RenderPanel handles borders internally, so only subtract divider
	available := p.width - dividerWidth
	sidebarWidth := p.sidebarWidth
	if sidebarWidth == 0 {
		sidebarWidth = available * 30 / 100
	}
	if sidebarWidth < 25 {
		sidebarWidth = 25
	}
	if sidebarWidth > available-40 {
		sidebarWidth = available - 40
	}
	mainWidth := available - sidebarWidth
	if mainWidth < 40 {
		mainWidth = 40
	}

	// Store for use by content renderers
	p.sidebarWidth = sidebarWidth

	// Determine if panes are active based on focus
	sidebarActive := p.activePane == PaneSidebar
	mainActive := p.activePane != PaneSidebar

	// Render sidebar (session list)
	sidebarContent := p.renderSidebarPane(innerHeight)

	// Render main pane (messages)
	mainContent := p.renderMainPane(mainWidth, innerHeight)

	// Apply gradient border styles
	leftPane := styles.RenderPanel(sidebarContent, sidebarWidth, paneHeight, sidebarActive)

	// Render visible divider
	divider := p.renderDivider(paneHeight)

	rightPane := styles.RenderPanel(mainContent, mainWidth, paneHeight, mainActive)

	// Only rebuild hit regions when dirty (td-ea784b03)
	mainX := sidebarWidth + dividerWidth
	if p.hitRegionsDirty {
		// Clear and re-register hit regions
		p.mouseHandler.HitMap.Clear()

		// Register hit regions (order matters: last = highest priority)
		// Sidebar region - lowest priority fallback
		p.mouseHandler.HitMap.AddRect(regionSidebar, 0, 0, sidebarWidth, p.height, nil)
		// Main pane region (after divider) - medium priority
		p.mouseHandler.HitMap.AddRect(regionMainPane, mainX, 0, mainWidth, p.height, nil)
		// Divider region - HIGH PRIORITY (registered after panes so it wins in overlap)
		dividerX := sidebarWidth
		dividerHitWidth := 3
		p.mouseHandler.HitMap.AddRect(regionPaneDivider, dividerX, 0, dividerHitWidth, p.height, nil)

		// Session item regions - HIGH PRIORITY
		p.registerSessionHitRegions(sidebarWidth, innerHeight)

		// Turn item regions - HIGHEST PRIORITY (registered last)
		p.registerTurnHitRegions(mainX+1, mainWidth-2, innerHeight)

		p.hitRegionsDirty = false
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)
}

// registerSessionHitRegions registers mouse hit regions for visible session items.
// This mirrors the rendering logic in renderSidebarPane/renderGroupedCompactSessions.
func (p *Plugin) registerSessionHitRegions(sidebarWidth, contentHeight int) {
	if p.filterMode {
		return // Filter menu is shown instead of sessions
	}

	sessions := p.visibleSessions()
	if len(sessions) == 0 {
		return
	}

	// Y offset: panel border (1) + title line (1) + optional search/filter line
	headerY := 2 // border + title
	if p.searchMode || p.filterActive {
		headerY = 3 // border + title + search/filter line
	}

	// X offset: panel border (1) + padding (1) = 2
	// The PanelActive/PanelInactive styles have Padding(0, 1) which adds horizontal padding
	hitX := 2

	// Hit region width: sidebarWidth - border(2) - padding(2) = sidebarWidth - 4
	hitWidth := sidebarWidth - 4
	if hitWidth < 10 {
		hitWidth = 10
	}

	// Track visual line position and visible session count
	lineCount := 0
	currentGroup := ""

	for i := p.scrollOff; i < len(sessions) && lineCount < contentHeight; i++ {
		session := sessions[i]

		// In grouped mode (not searching), account for group headers and spacers
		if !p.searchMode {
			sessionGroup := getSessionGroup(session.UpdatedAt)
			if sessionGroup != currentGroup {
				// Spacer before Yesterday/This Week (except first group)
				if currentGroup != "" && (sessionGroup == "Yesterday" || sessionGroup == "This Week") {
					lineCount++
					if lineCount >= contentHeight {
						break
					}
				}
				// Group header line
				currentGroup = sessionGroup
				lineCount++
				if lineCount >= contentHeight {
					break
				}
			}
		}

		// Register hit region for this session
		itemY := headerY + lineCount
		p.mouseHandler.HitMap.AddRect(regionSessionItem, hitX, itemY, hitWidth, 1, i)
		lineCount++
	}
}

// registerTurnHitRegions registers mouse hit regions for visible turn items in the main pane.
func (p *Plugin) registerTurnHitRegions(mainX, contentWidth, contentHeight int) {
	if p.detailMode {
		return
	}

	// Y offset: panel border (1) + header lines (4: title, stats, resume cmd, separator)
	headerY := 5
	currentY := headerY

	if p.turnViewMode {
		// Turn view: register hit regions for turns
		if len(p.turns) == 0 {
			return
		}
		for i := p.turnScrollOff; i < len(p.turns); i++ {
			turn := p.turns[i]
			turnHeight := p.calculateTurnHeight(turn, contentWidth)

			if currentY+turnHeight > contentHeight+headerY {
				break
			}

			p.mouseHandler.HitMap.AddRect(regionTurnItem, mainX, currentY, contentWidth, turnHeight, i)
			currentY += turnHeight
		}
	} else {
		// Conversation flow: register hit regions for messages
		if len(p.messages) == 0 {
			return
		}
		p.registerMessageHitRegions(mainX, contentWidth, contentHeight, headerY)
	}
}

// registerMessageHitRegions registers mouse hit regions for visible messages in conversation flow.
// Uses visibleMsgRanges populated during renderConversationFlow for accurate positioning.
func (p *Plugin) registerMessageHitRegions(mainX, contentWidth, contentHeight, headerY int) {
	for _, mr := range p.visibleMsgRanges {
		// Convert relative line position to screen Y coordinate
		screenY := headerY + mr.StartLine
		if mr.LineCount > 0 && screenY < headerY+contentHeight {
			p.mouseHandler.HitMap.AddRect(regionMessageItem, mainX, screenY, contentWidth, mr.LineCount, mr.MsgIdx)
		}
	}
}

// calculateTurnHeight returns the number of lines a turn will occupy when rendered.
func (p *Plugin) calculateTurnHeight(turn Turn, maxWidth int) int {
	height := 1 // header line always present
	if turn.ThinkingTokens > 0 {
		height++
	}
	content := turn.Preview(maxWidth - 7)
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.TrimSpace(content)
	if content != "" {
		height++
	}
	if turn.ToolCount > 0 {
		height++
	}
	return height
}

// renderDivider renders the visible divider between panes.
func (p *Plugin) renderDivider(height int) string {
	dividerStyle := lipgloss.NewStyle().
		Foreground(styles.BorderNormal).
		MarginTop(1) // Shifts down to align with pane content

	var sb strings.Builder
	for i := 0; i < height; i++ {
		sb.WriteString("│")
		if i < height-1 {
			sb.WriteString("\n")
		}
	}
	return dividerStyle.Render(sb.String())
}

// renderSidebarPane renders the session list for the sidebar.
func (p *Plugin) renderSidebarPane(height int) string {
	var sb strings.Builder

	sessions := p.visibleSessions()

	// Content width = sidebar width - border (2) - padding (2) = 4
	contentWidth := p.sidebarWidth - 4
	if contentWidth < 15 {
		contentWidth = 15
	}

	// Header with count
	countStr := fmt.Sprintf("%d", len(p.sessions))
	if p.searchMode && p.searchQuery != "" {
		countStr = fmt.Sprintf("%d/%d", len(sessions), len(p.sessions))
	}
	// Truncate count if needed
	maxCountLen := contentWidth - len("Sessions ")
	if maxCountLen > 0 && len(countStr) > maxCountLen {
		countStr = countStr[:maxCountLen]
	}
	sb.WriteString(styles.Title.Render("Sessions"))
	sb.WriteString(styles.Muted.Render(" " + countStr))
	sb.WriteString("\n")

	linesUsed := 1

	// Search bar (if in search mode)
	if p.searchMode {
		searchLine := fmt.Sprintf("/%s█", p.searchQuery)
		if len(searchLine) > contentWidth {
			searchLine = searchLine[:contentWidth]
		}
		sb.WriteString(styles.StatusInProgress.Render(searchLine))
		sb.WriteString("\n")
		linesUsed++
	} else if p.filterActive {
		filterStr := p.filters.String()
		if len(filterStr) > contentWidth {
			filterStr = filterStr[:contentWidth-3] + "..."
		}
		sb.WriteString(styles.Muted.Render(filterStr))
		sb.WriteString("\n")
		linesUsed++
	}

	// Filter menu (if in filter mode)
	if p.filterMode {
		sb.WriteString(p.renderFilterMenu(height - linesUsed))
		return sb.String()
	}

	// Session list
	if len(sessions) == 0 {
		if p.searchMode {
			sb.WriteString(styles.Muted.Render("No matching sessions"))
		} else {
			sb.WriteString(styles.Muted.Render("No sessions"))
		}
		return sb.String()
	}

	// Render sessions
	contentHeight := height - linesUsed
	if contentHeight < 1 {
		contentHeight = 1
	}
	if !p.searchMode {
		groups := GroupSessionsByTime(sessions)
		p.renderGroupedCompactSessions(&sb, groups, contentHeight, contentWidth)
	} else {
		end := p.scrollOff + contentHeight
		if end > len(sessions) {
			end = len(sessions)
		}

		for i := p.scrollOff; i < end; i++ {
			session := sessions[i]
			selected := i == p.cursor
			sb.WriteString(p.renderCompactSessionRow(session, selected, contentWidth))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (p *Plugin) renderGroupedCompactSessions(sb *strings.Builder, groups []SessionGroup, contentHeight int, contentWidth int) {
	sessions := p.visibleSessions()

	lineCount := 0
	currentGroup := ""

	for i := p.scrollOff; i < len(sessions) && lineCount < contentHeight; i++ {
		session := sessions[i]
		sessionGroup := getSessionGroup(session.UpdatedAt)

		if sessionGroup != currentGroup {
			if currentGroup != "" && (sessionGroup == "Yesterday" || sessionGroup == "This Week") {
				sb.WriteString("\n")
				lineCount++
				if lineCount >= contentHeight {
					break
				}
			}

			currentGroup = sessionGroup

			// Find group count
			groupStats := ""
			for _, g := range groups {
				if g.Label == sessionGroup {
					groupStats = fmt.Sprintf(" (%d)", g.Summary.SessionCount)
					break
				}
			}

			groupHeader := sessionGroup + groupStats
			if len(groupHeader) > contentWidth {
				groupHeader = groupHeader[:contentWidth]
			}
			sb.WriteString(styles.Code.Render(groupHeader))
			sb.WriteString("\n")
			lineCount++
			if lineCount >= contentHeight {
				break
			}
		}

		selected := i == p.cursor
		sb.WriteString(p.renderCompactSessionRow(session, selected, contentWidth))
		sb.WriteString("\n")
		lineCount++
	}
}

// renderCompactSessionRow renders a compact session row for the sidebar.
// Format: [active] [icon] [worktree] Session title...              12m  45k
func (p *Plugin) renderCompactSessionRow(session adapter.Session, selected bool, maxWidth int) string {
	// Get badge text for width calculations (plain text length)
	badgeText := adapterBadgeText(session)

	// Format worktree badge if session is from a different worktree
	worktreeBadge := ""
	if session.WorktreeName != "" {
		// Truncate long worktree names to keep UI clean (rune-safe for Unicode)
		wtName := session.WorktreeName
		runes := []rune(wtName)
		if len(runes) > 12 {
			wtName = string(runes[:9]) + "..."
		}
		worktreeBadge = "[" + wtName + "]"
	}

	// Format duration - only if we have data
	lengthCol := ""
	if session.Duration > 0 {
		lengthCol = formatSessionDuration(session.Duration)
	}

	// Format token count - only if we have data
	tokenCol := ""
	if session.TotalTokens > 0 {
		tokenCol = formatK(session.TotalTokens)
	}

	// Calculate right column width (only for columns that have data)
	rightColWidth := 0
	if lengthCol != "" {
		rightColWidth += len(lengthCol)
	}
	if tokenCol != "" {
		if rightColWidth > 0 {
			rightColWidth += 1 // space between columns
		}
		rightColWidth += len(tokenCol)
	}

	// Calculate prefix length for width calculations
	// active(1) + badge + space + worktree + space (if worktree)
	prefixLen := 1 + len(badgeText) + 1
	if worktreeBadge != "" {
		prefixLen += len(worktreeBadge) + 1 // badge + space
	}
	if session.IsSubAgent {
		prefixLen += 2 // extra indent for sub-agents
	}
	// Add right column width plus spacing if present
	if rightColWidth > 0 {
		prefixLen += rightColWidth + 2 // space before + space after
	}

	// Session name/ID
	name := session.Name
	if name == "" {
		name = shortID(session.ID)
	}

	// Calculate available width for name
	nameWidth := maxWidth - prefixLen
	if nameWidth < 5 {
		nameWidth = 5
	}

	// Truncate name to fit (rune-safe for Unicode)
	if runes := []rune(name); len(runes) > nameWidth {
		name = string(runes[:nameWidth-3]) + "..."
	}

	// Calculate padding for right-aligned stats
	visibleLen := 0
	if session.IsSubAgent {
		visibleLen += 2
	}
	visibleLen += 1                              // indicator
	visibleLen += len(badgeText) + 1 + len(name) // badge + space + name
	if worktreeBadge != "" {
		visibleLen += len(worktreeBadge) + 1 // worktree badge + space
	}
	padding := maxWidth - visibleLen - rightColWidth - 1
	if padding < 0 {
		padding = 0
	}

	// Build the row with styling
	var sb strings.Builder

	// Sub-agent indent
	if session.IsSubAgent {
		sb.WriteString("  ")
	}

	// Activity indicator with colors
	if session.IsActive {
		sb.WriteString(styles.StatusInProgress.Render("●"))
	} else if session.IsSubAgent {
		sb.WriteString(styles.Muted.Render("↳"))
	} else {
		sb.WriteString(" ")
	}

	// Colored adapter icon + worktree badge + name based on session type
	if session.IsSubAgent {
		// Sub-agents: muted styling
		sb.WriteString(styles.Muted.Render(badgeText))
		sb.WriteString(" ")
		if worktreeBadge != "" {
			sb.WriteString(styles.Muted.Render(worktreeBadge))
			sb.WriteString(" ")
		}
		sb.WriteString(styles.Subtitle.Render(name))
	} else {
		// Top-level: use colored adapter icon
		sb.WriteString(renderAdapterIcon(session))
		sb.WriteString(" ")
		if worktreeBadge != "" {
			// Cyan/teal color for worktree badge to stand out
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#14B8A6")).Render(worktreeBadge))
			sb.WriteString(" ")
		}
		sb.WriteString(styles.Body.Render(name))
	}

	// Padding and right-aligned stats (only if we have data)
	if rightColWidth > 0 && padding > 0 {
		sb.WriteString(strings.Repeat(" ", padding))
		sb.WriteString(" ")
		if lengthCol != "" {
			if session.IsSubAgent {
				sb.WriteString(styles.Muted.Render(lengthCol))
			} else {
				sb.WriteString(styles.Subtitle.Render(lengthCol))
			}
		}
		if tokenCol != "" {
			if lengthCol != "" {
				sb.WriteString(" ")
			}
			sb.WriteString(styles.Subtle.Render(tokenCol))
		}
	}

	row := sb.String()

	// For selected rows, build plain text version with background highlight
	if selected {
		var plain strings.Builder
		if session.IsSubAgent {
			plain.WriteString("  ")
		}
		if session.IsActive {
			plain.WriteString("●")
		} else if session.IsSubAgent {
			plain.WriteString("↳")
		} else {
			plain.WriteString(" ")
		}
		plain.WriteString(badgeText)
		plain.WriteString(" ")
		if worktreeBadge != "" {
			plain.WriteString(worktreeBadge)
			plain.WriteString(" ")
		}
		plain.WriteString(name)
		if rightColWidth > 0 && padding > 0 {
			plain.WriteString(strings.Repeat(" ", padding))
			plain.WriteString(" ")
			if lengthCol != "" {
				plain.WriteString(lengthCol)
			}
			if tokenCol != "" {
				if lengthCol != "" {
					plain.WriteString(" ")
				}
				plain.WriteString(tokenCol)
			}
		}
		plainRow := plain.String()
		// Pad to full width for proper background highlight
		if len(plainRow) < maxWidth {
			plainRow += strings.Repeat(" ", maxWidth-len(plainRow))
		}
		return styles.ListItemSelected.Render(plainRow)
	}

	return row
}

// renderMainPane renders the message list for the main pane.
func (p *Plugin) renderMainPane(paneWidth, height int) string {
	// Content width = pane width - border (2) - padding (2) = 4
	contentWidth := paneWidth - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	if p.selectedSession == "" {
		return styles.Muted.Render("Select a session to view messages")
	}

	// If in detail mode, render the turn detail instead of turn list
	if p.detailMode && p.detailTurn != nil {
		return p.renderDetailPaneContent(contentWidth, height)
	}

	var sb strings.Builder

	// Find session info
	var session *adapter.Session
	for i := range p.sessions {
		if p.sessions[i].ID == p.selectedSession {
			session = &p.sessions[i]
			break
		}
	}

	// Header Line 1: Adapter icon + Session name
	sessionName := shortID(p.selectedSession)
	if session != nil && session.Name != "" {
		sessionName = session.Name
	}

	// Calculate max length for session name (leave room for icon)
	maxSessionLen := contentWidth - 4
	if maxSessionLen < 10 {
		maxSessionLen = 10
	}
	if len(sessionName) > maxSessionLen {
		sessionName = sessionName[:maxSessionLen-3] + "..."
	}

	// Build header with colored adapter icon
	if session != nil {
		sb.WriteString(renderAdapterIcon(*session))
		sb.WriteString(" ")
	}
	sb.WriteString(styles.Title.Render(sessionName))
	sb.WriteString("\n")

	// Header Line 2: Model badge │ msgs │ tokens │ cost │ date
	if p.sessionSummary != nil {
		s := p.sessionSummary

		// Build stats with model badge
		var statsParts []string

		// Model badge (colorful)
		if s.PrimaryModel != "" {
			badge := renderModelBadge(s.PrimaryModel)
			if badge != "" {
				statsParts = append(statsParts, badge)
			}
		} else if session != nil {
			// Fallback to adapter short name
			shortName := adapterShortName(session)
			if shortName != "" {
				statsParts = append(statsParts, styles.Code.Render(shortName))
			}
		}

		// Message count
		statsParts = append(statsParts, fmt.Sprintf("%d msgs", s.MessageCount))

		// Token flow
		statsParts = append(statsParts, fmt.Sprintf("%s→%s", formatK(s.TotalTokensIn), formatK(s.TotalTokensOut)))

		// Cost estimate
		if session != nil && session.EstCost > 0 {
			statsParts = append(statsParts, formatCost(session.EstCost))
		}

		// Last updated
		if session != nil && !session.UpdatedAt.IsZero() {
			statsParts = append(statsParts, session.UpdatedAt.Local().Format("Jan 02 15:04"))
		}

		statsLine := strings.Join(statsParts, " │ ")
		// Check if we need to truncate (accounting for ANSI codes in badge)
		if lipgloss.Width(statsLine) > contentWidth {
			// Rebuild without badge for narrow widths
			statsParts = statsParts[1:] // Remove badge
			statsLine = strings.Join(statsParts, " │ ")
		}
		sb.WriteString(styles.Muted.Render(statsLine))
		sb.WriteString("\n")
	}

	// Header Line 3: Resume command with copy hint
	if session != nil {
		resumeCmd := resumeCommand(session)
		if resumeCmd != "" {
			maxCmdLen := contentWidth - 12 // Leave room for copy hint
			if len(resumeCmd) > maxCmdLen {
				resumeCmd = resumeCmd[:maxCmdLen-3] + "..."
			}
			sb.WriteString(styles.Code.Render(resumeCmd))
			sb.WriteString("  ")
			sb.WriteString(styles.Subtle.Render("[Y:copy]"))
			sb.WriteString("\n")
		}
	}

	// Pagination indicator (td-313ea851)
	if p.totalMessages > maxMessagesInMemory {
		startIdx := p.totalMessages - p.messageOffset - len(p.messages) + 1
		endIdx := p.totalMessages - p.messageOffset
		if startIdx < 1 {
			startIdx = 1
		}
		pageInfo := fmt.Sprintf("Showing %d-%d of %d messages", startIdx, endIdx, p.totalMessages)
		if p.hasOlderMsgs {
			pageInfo += " [p:older"
			if p.messageOffset > 0 {
				pageInfo += " n:newer"
			}
			pageInfo += "]"
		} else if p.messageOffset > 0 {
			pageInfo += " [n:newer]"
		}
		if len(pageInfo) > contentWidth {
			pageInfo = pageInfo[:contentWidth-3] + "..."
		}
		sb.WriteString(styles.StatusModified.Render(pageInfo))
		sb.WriteString("\n")
	}

	sepWidth := contentWidth
	if sepWidth > 60 {
		sepWidth = 60
	}
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", sepWidth)))
	sb.WriteString("\n")

	contentHeight := height - 4 // Account for header lines
	// Adjust for pagination indicator if visible (td-313ea851)
	if p.totalMessages > maxMessagesInMemory {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Check for empty/loading state
	if len(p.messages) == 0 && len(p.turns) == 0 {
		if session != nil && session.MessageCount == 0 {
			sb.WriteString(styles.Muted.Render("No messages (metadata only)"))
		} else {
			sb.WriteString(styles.Muted.Render("Loading messages..."))
		}
		return sb.String()
	}

	if p.turnViewMode {
		// Turn-based view (metadata-focused)
		if len(p.turns) == 0 {
			sb.WriteString(styles.Muted.Render("No turns"))
			return sb.String()
		}
		lineCount := 0
		for i := p.turnScrollOff; i < len(p.turns) && lineCount < contentHeight; i++ {
			turn := p.turns[i]
			lines := p.renderCompactTurn(turn, i, contentWidth)
			for _, line := range lines {
				if lineCount >= contentHeight {
					break
				}
				sb.WriteString(line)
				sb.WriteString("\n")
				lineCount++
			}
		}
	} else {
		// Conversation flow view (content-focused, default)
		lines := p.renderConversationFlow(contentWidth, contentHeight)
		for _, line := range lines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderDetailPaneContent renders the turn detail in the right pane (two-pane mode).
func (p *Plugin) renderDetailPaneContent(contentWidth, height int) string {
	var sb strings.Builder

	if p.detailTurn == nil {
		return styles.Muted.Render("No turn selected")
	}

	turn := p.detailTurn
	msgCount := len(turn.Messages)

	// Header: Turn Role (with message count if > 1)
	roleLabel := turn.Role
	if msgCount > 1 {
		roleLabel = fmt.Sprintf("%s (%d messages)", turn.Role, msgCount)
	}
	header := fmt.Sprintf("%s Turn", strings.Title(roleLabel))
	if len(header) > contentWidth-10 {
		header = header[:contentWidth-13] + "..."
	}
	sb.WriteString(styles.Title.Render(header))
	sb.WriteString("  ")
	sb.WriteString(styles.Muted.Render("[esc]"))
	sb.WriteString("\n")

	// Stats line
	var stats []string
	if turn.TotalTokensIn > 0 || turn.TotalTokensOut > 0 {
		stats = append(stats, fmt.Sprintf("%s→%s tokens", formatK(turn.TotalTokensIn), formatK(turn.TotalTokensOut)))
	}
	if turn.ThinkingTokens > 0 {
		stats = append(stats, fmt.Sprintf("%s thinking", formatK(turn.ThinkingTokens)))
	}
	if turn.ToolCount > 0 {
		stats = append(stats, fmt.Sprintf("%d tools", turn.ToolCount))
	}
	if len(stats) > 0 {
		statsLine := strings.Join(stats, " │ ")
		if len(statsLine) > contentWidth {
			statsLine = statsLine[:contentWidth-3] + "..."
		}
		sb.WriteString(styles.Muted.Render(statsLine))
		sb.WriteString("\n")
	}

	// Separator
	sepWidth := contentWidth
	if sepWidth > 60 {
		sepWidth = 60
	}
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", sepWidth)))
	sb.WriteString("\n")

	// Build content lines for all messages in turn
	var contentLines []string

	for msgIdx, msg := range turn.Messages {
		// Message separator (except for first)
		if msgIdx > 0 {
			contentLines = append(contentLines, "")
			contentLines = append(contentLines, styles.Muted.Render(fmt.Sprintf("── Message %d/%d ──", msgIdx+1, msgCount)))
			contentLines = append(contentLines, "")
		}

		// Thinking blocks
		for i, tb := range msg.ThinkingBlocks {
			contentLines = append(contentLines, styles.Code.Render(fmt.Sprintf("Thinking %d (%d tokens)", i+1, tb.TokenCount)))
			// Wrap thinking content
			thinkingLines := wrapText(tb.Content, contentWidth-2)
			for _, line := range thinkingLines {
				contentLines = append(contentLines, styles.Muted.Render(line))
			}
			contentLines = append(contentLines, "")
		}

		// Main content
		if msg.Content != "" {
			// Render markdown content
			msgLines := p.renderContent(msg.Content, contentWidth-2)
			for _, line := range msgLines {
				contentLines = append(contentLines, line) // Glamour already styled
			}
			contentLines = append(contentLines, "")
		}

		// Tool uses
		if len(msg.ToolUses) > 0 {
			contentLines = append(contentLines, styles.Subtitle.Render("Tools:"))
			for _, tu := range msg.ToolUses {
				toolLine := tu.Name
				if filePath := extractFilePath(tu.Input); filePath != "" {
					toolLine += ": " + filePath
				}
				if len(toolLine) > contentWidth-2 {
					toolLine = toolLine[:contentWidth-5] + "..."
				}
				contentLines = append(contentLines, styles.Code.Render("  "+toolLine))
			}
			contentLines = append(contentLines, "")
		}
	}

	// Apply scroll offset
	headerLines := 3 // title + stats + separator
	contentHeight := height - headerLines
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Clamp scroll
	maxScroll := len(contentLines) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.detailScroll > maxScroll {
		p.detailScroll = maxScroll
	}
	if p.detailScroll < 0 {
		p.detailScroll = 0
	}

	// Reserve space for scroll indicators (up to 2 lines)
	indicatorLines := 0
	if maxScroll > 0 {
		if p.detailScroll > 0 {
			indicatorLines++
		}
		if p.detailScroll < maxScroll {
			indicatorLines++
		}
	}
	displayHeight := contentHeight - indicatorLines
	if displayHeight < 1 {
		displayHeight = 1
	}

	start := p.detailScroll
	end := start + displayHeight
	if end > len(contentLines) {
		end = len(contentLines)
	}

	for i := start; i < end; i++ {
		sb.WriteString(contentLines[i])
		sb.WriteString("\n")
	}

	// Scroll indicators (space already reserved)
	if maxScroll > 0 {
		if p.detailScroll > 0 {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("↑ %d more above", p.detailScroll)))
			sb.WriteString("\n")
		}
		remaining := len(contentLines) - end
		if remaining > 0 {
			sb.WriteString(styles.Muted.Render(fmt.Sprintf("↓ %d more below", remaining)))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderCompactTurn renders a turn (grouped messages) in compact format for two-pane view.
func (p *Plugin) renderCompactTurn(turn Turn, turnIndex int, maxWidth int) []string {
	var lines []string
	selected := turnIndex == p.turnCursor

	// Header line: [timestamp] role (N msgs) tokens
	ts := turn.FirstTimestamp()

	// Build stats string
	msgCount := len(turn.Messages)
	var stats []string
	if msgCount > 1 {
		stats = append(stats, fmt.Sprintf("%d msgs", msgCount))
	}
	if turn.TotalTokensIn > 0 || turn.TotalTokensOut > 0 {
		stats = append(stats, fmt.Sprintf("%s→%s", formatK(turn.TotalTokensIn), formatK(turn.TotalTokensOut)))
	}
	statsStr := ""
	if len(stats) > 0 {
		statsStr = " (" + strings.Join(stats, ", ") + ")"
	}

	// Build header line
	if selected {
		// For selected: plain text with background highlight
		headerContent := fmt.Sprintf("[%s] %s%s", ts, turn.Role, statsStr)
		lines = append(lines, p.styleTurnLine(headerContent, true, maxWidth))
	} else {
		// For unselected: colored role badge with muted styling
		var roleStyle lipgloss.Style
		if turn.Role == "user" {
			roleStyle = styles.StatusInProgress
		} else {
			roleStyle = styles.StatusStaged
		}
		styledHeader := fmt.Sprintf("[%s] %s%s",
			styles.Muted.Render(ts),
			roleStyle.Render(turn.Role),
			styles.Muted.Render(statsStr))
		lines = append(lines, styledHeader)
	}

	// Thinking indicator (aggregate) - indented under header
	if turn.ThinkingTokens > 0 {
		thinkingLine := fmt.Sprintf("   ├─ [thinking] %s tokens", formatK(turn.ThinkingTokens))
		if len(thinkingLine) > maxWidth {
			thinkingLine = thinkingLine[:maxWidth-3] + "..."
		}
		lines = append(lines, p.styleTurnLine(thinkingLine, selected, maxWidth))
	}

	// Content preview from first meaningful message - indented under header
	content := turn.Preview(maxWidth - 5)
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.TrimSpace(content)
	if content != "" {
		contentLine := "   " + content
		lines = append(lines, p.styleTurnLine(contentLine, selected, maxWidth))
	}

	// Tool uses (aggregate) - indented under header
	if turn.ToolCount > 0 {
		toolLine := fmt.Sprintf("   └─ %d tools", turn.ToolCount)
		lines = append(lines, p.styleTurnLine(toolLine, selected, maxWidth))
	}

	return lines
}

// styleTurnLine applies selection highlighting or default muted styling to a turn line.
func (p *Plugin) styleTurnLine(content string, selected bool, maxWidth int) string {
	if selected {
		// Pad to full width for proper background highlighting
		if len(content) < maxWidth {
			content += strings.Repeat(" ", maxWidth-len(content))
		}
		return styles.ListItemSelected.Render(content)
	}
	return styles.Muted.Render(content)
}

// renderCompactMessage renders a message in compact format for two-pane view.
func (p *Plugin) renderCompactMessage(msg adapter.Message, msgIndex int, maxWidth int) []string {
	var lines []string

	// Header line: [timestamp] role  tokens
	ts := msg.Timestamp.Local().Format("15:04")
	var roleStyle lipgloss.Style
	if msg.Role == "user" {
		roleStyle = styles.StatusInProgress
	} else {
		roleStyle = styles.StatusStaged
	}

	// Cursor indicator
	var styledCursor string
	if msgIndex == p.msgCursor {
		styledCursor = styles.ListCursor.Render("> ")
	} else {
		styledCursor = "  "
	}

	// Token info - truncate if needed
	tokens := ""
	if msg.OutputTokens > 0 || msg.InputTokens > 0 {
		tokens = fmt.Sprintf(" (%s→%s)", formatK(msg.InputTokens), formatK(msg.OutputTokens))
	}

	// Calculate if we need to truncate role (rune-safe for Unicode)
	role := msg.Role
	roleRunes := []rune(role)
	// Account for: cursor(2) + [](2) + ts(5) + space(1) + role + tokens
	usedWidth := 2 + 2 + len(ts) + 1 + len(roleRunes) + len(tokens)
	if usedWidth > maxWidth && len(roleRunes) > 4 {
		role = string(roleRunes[:4])
	}

	// Build styled header
	styledHeader := styledCursor + fmt.Sprintf("[%s] %s%s",
		styles.Muted.Render(ts),
		roleStyle.Render(role),
		styles.Muted.Render(tokens))
	lines = append(lines, styledHeader)

	// Thinking indicator
	if len(msg.ThinkingBlocks) > 0 {
		var totalTokens int
		for _, tb := range msg.ThinkingBlocks {
			totalTokens += tb.TokenCount
		}
		thinkingLine := fmt.Sprintf("  ├─ [thinking] %s tokens", formatK(totalTokens))
		if len(thinkingLine) > maxWidth {
			thinkingLine = thinkingLine[:maxWidth-3] + "..."
		}
		lines = append(lines, styles.Muted.Render(thinkingLine))
	}

	// Content preview (truncated, rune-safe for Unicode)
	content := msg.Content
	content = strings.ReplaceAll(content, "\n", " ")
	content = strings.TrimSpace(content)
	contentMaxLen := maxWidth - 4 // Account for "   " prefix
	if contentMaxLen < 10 {
		contentMaxLen = 10
	}
	if runes := []rune(content); len(runes) > contentMaxLen {
		content = string(runes[:contentMaxLen-3]) + "..."
	}
	if content != "" {
		lines = append(lines, "  "+styles.Body.Render(content))
	}

	// Tool uses (compact)
	if len(msg.ToolUses) > 0 {
		toolLine := fmt.Sprintf("  └─ %d tools", len(msg.ToolUses))
		lines = append(lines, styles.Code.Render(toolLine))
	}

	return lines
}

// renderConversationFlow renders messages as a scrollable chat thread (Claude Code web UI style).
func (p *Plugin) renderConversationFlow(contentWidth, height int) []string {
	// Clear previous tracking data
	p.visibleMsgRanges = p.visibleMsgRanges[:0]
	p.msgLinePositions = p.msgLinePositions[:0]

	if len(p.messages) == 0 {
		return []string{styles.Muted.Render("No messages")}
	}

	var allLines []string
	prevRole := ""

	for msgIdx, msg := range p.messages {
		// Skip user messages that are just tool results (they'll be shown inline with tool_use)
		if p.isToolResultOnlyMessage(msg) {
			continue
		}

		// Add subtle turn separator when role changes (user ↔ assistant)
		if prevRole != "" && prevRole != msg.Role {
			// Create a subtle visual break between turns
			sepWidth := contentWidth / 3
			if sepWidth > 20 {
				sepWidth = 20
			}
			separator := strings.Repeat("─", sepWidth)
			allLines = append(allLines, styles.Subtle.Render("  "+separator))
		}
		prevRole = msg.Role

		// Track where this message starts
		startLine := len(allLines)

		// Render message bubble
		msgLines := p.renderMessageBubble(msg, msgIdx, contentWidth)
		allLines = append(allLines, msgLines...)

		allLines = append(allLines, "") // Gap between messages

		// Store position info for scroll calculations (include gap line in count)
		p.msgLinePositions = append(p.msgLinePositions, msgLinePos{
			MsgIdx:    msgIdx,
			StartLine: startLine,
			LineCount: len(msgLines) + 1, // +1 for gap line
		})
	}

	// Apply scroll offset
	maxScroll := len(allLines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.messageScroll > maxScroll {
		p.messageScroll = maxScroll
	}
	if p.messageScroll < 0 {
		p.messageScroll = 0
	}

	start := p.messageScroll
	end := start + height
	if end > len(allLines) {
		end = len(allLines)
	}

	if start >= len(allLines) {
		return []string{}
	}

	// Calculate visible ranges for hit region registration
	// screenLine is relative to content area (0 = first visible line)
	for _, mp := range p.msgLinePositions {
		msgEnd := mp.StartLine + mp.LineCount
		// Check if message is visible in the scroll window
		if msgEnd <= start {
			continue // Message is entirely before scroll window
		}
		if mp.StartLine >= end {
			break // Message is entirely after scroll window
		}

		// Calculate visible portion
		visibleStart := mp.StartLine - start
		if visibleStart < 0 {
			visibleStart = 0
		}
		visibleEnd := msgEnd - start
		if visibleEnd > height {
			visibleEnd = height
		}

		if visibleEnd > visibleStart {
			p.visibleMsgRanges = append(p.visibleMsgRanges, msgLineRange{
				MsgIdx:    mp.MsgIdx,
				StartLine: visibleStart,
				LineCount: visibleEnd - visibleStart,
			})
		}
	}

	return allLines[start:end]
}

// renderMessageBubble renders a single message as a chat bubble with content blocks.
func (p *Plugin) renderMessageBubble(msg adapter.Message, msgIndex int, maxWidth int) []string {
	var lines []string
	selected := msgIndex == p.messageCursor

	// Header: timestamp + role + model badge + token flow
	ts := msg.Timestamp.Local().Format("15:04")

	// Cursor indicator for selected message
	cursorPrefix := "  "
	if selected {
		cursorPrefix = "> "
	}

	var headerLine string
	if selected {
		// For selected messages, use plain text (no colored backgrounds) for consistent highlighting
		if msg.Role == "user" {
			headerLine = fmt.Sprintf("%s[%s] you", cursorPrefix, ts)
		} else {
			headerLine = fmt.Sprintf("%s[%s] claude", cursorPrefix, ts)
			// Add plain model name
			if msg.Model != "" {
				short := modelShortName(msg.Model)
				if short != "" {
					headerLine += " " + short
				}
			}
			// Add plain token flow
			if msg.InputTokens > 0 || msg.OutputTokens > 0 {
				headerLine += " " + fmt.Sprintf("%s→%s", formatK(msg.InputTokens), formatK(msg.OutputTokens))
			}
		}
	} else {
		// For non-selected messages, use colorful styling
		if msg.Role == "user" {
			headerLine = fmt.Sprintf("%s[%s] %s", cursorPrefix, ts, styles.StatusInProgress.Render("you"))
		} else {
			headerLine = fmt.Sprintf("%s[%s] %s", cursorPrefix, ts, styles.StatusStaged.Render("claude"))

			// Add colorful model badge
			if msg.Model != "" {
				badge := renderModelBadge(msg.Model)
				if badge != "" {
					headerLine += " " + badge
				}
			}

			// Add token flow indicator
			tokenFlow := renderTokenFlow(msg.InputTokens, msg.OutputTokens)
			if tokenFlow != "" {
				headerLine += " " + tokenFlow
			}
		}
	}
	lines = append(lines, headerLine)

	// Render content blocks (same for selected and non-selected)
	if len(msg.ContentBlocks) > 0 {
		blockLines := p.renderContentBlocks(msg, maxWidth-4)
		for _, line := range blockLines {
			lines = append(lines, "    "+line)
		}
	} else if msg.Content != "" {
		contentLines := p.renderMessageContent(msg.Content, msg.ID, maxWidth-4)
		for _, line := range contentLines {
			lines = append(lines, "    "+line)
		}
	}

	// Apply selection highlighting if needed
	if selected {
		var styledLines []string
		for _, line := range lines {
			// Strip any existing background colors so selection bg shows through
			line = stripANSIBackground(line)
			// Use visible width (not byte length) for proper padding
			visibleWidth := lipgloss.Width(line)
			if visibleWidth < maxWidth {
				line += strings.Repeat(" ", maxWidth-visibleWidth)
			}
			styledLines = append(styledLines, styles.ListItemSelected.Render(line))
		}
		return styledLines
	}

	return lines
}

// renderContentBlocks renders the structured content blocks for a message.
func (p *Plugin) renderContentBlocks(msg adapter.Message, maxWidth int) []string {
	var lines []string

	for _, block := range msg.ContentBlocks {
		switch block.Type {
		case "text":
			textLines := p.renderMessageContent(block.Text, msg.ID, maxWidth)
			lines = append(lines, textLines...)

		case "thinking":
			thinkingLines := p.renderThinkingBlock(block, msg.ID, maxWidth)
			lines = append(lines, thinkingLines...)

		case "tool_use":
			toolLines := p.renderToolUseBlock(block, maxWidth)
			lines = append(lines, toolLines...)

		case "tool_result":
			// Tool results are rendered inline with tool_use via ToolOutput
			// Skip standalone tool_result blocks in the flow
			continue
		}
	}

	return lines
}

// renderMessageContent renders text content with expand/collapse for long messages.
// Uses render cache (td-8910b218) to avoid re-rendering unchanged content.
func (p *Plugin) renderMessageContent(content string, msgID string, maxWidth int) []string {
	if content == "" {
		return nil
	}

	// Check if content is "short" (can display inline)
	lineCount := strings.Count(content, "\n") + 1
	isShort := len(content) <= ShortMessageCharLimit && lineCount <= ShortMessageLineLimit

	expanded := p.expandedMessages[msgID]

	// Check cache (td-8910b218)
	if cached, ok := p.getCachedRender(msgID, maxWidth, expanded); ok {
		return strings.Split(cached, "\n")
	}

	var result []string
	if isShort || expanded {
		// Show full content
		result = p.renderContent(content, maxWidth)
	} else {
		// Collapsed: show preview with toggle hint (rune-safe for Unicode)
		preview := content
		if runes := []rune(preview); len(runes) > CollapsedPreviewChars {
			preview = string(runes[:CollapsedPreviewChars])
		}
		// Clean up preview (no partial lines)
		preview = strings.ReplaceAll(preview, "\n", " ")
		preview = strings.TrimSpace(preview)
		if len(preview) < len(content) {
			preview += "..."
		}
		result = wrapText(preview, maxWidth)
	}

	// Store in cache (td-8910b218)
	p.setCachedRender(msgID, maxWidth, expanded, strings.Join(result, "\n"))
	return result
}

// renderThinkingBlock renders a thinking block (collapsed by default).
// Uses render cache (td-8910b218) to avoid re-rendering unchanged content.
// Shows preview when collapsed, full content with │ prefix when expanded.
func (p *Plugin) renderThinkingBlock(block adapter.ContentBlock, msgID string, maxWidth int) []string {
	expanded := p.expandedThinking[msgID]

	// Use cache key with "thinking_" prefix to distinguish from content cache
	thinkingCacheID := "thinking_" + msgID
	if cached, ok := p.getCachedRender(thinkingCacheID, maxWidth, expanded); ok {
		return strings.Split(cached, "\n")
	}

	var lines []string

	// Light purple style for thinking blocks
	thinkingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Italic(true)

	thinkingIcon := "◈"
	tokenStr := formatK(block.TokenCount)

	if expanded {
		// Expanded: show ▼ indicator and full content
		header := fmt.Sprintf("%s thinking (%s tokens) ▼", thinkingIcon, tokenStr)
		lines = append(lines, thinkingStyle.Render(header))

		// Render thinking content with │ prefix for visual distinction
		thinkingLines := wrapText(block.Text, maxWidth-4)
		for _, line := range thinkingLines {
			lines = append(lines, styles.Muted.Render("  │ "+line))
		}
	} else {
		// Collapsed: show ▶ indicator and preview
		preview := block.Text
		// Clean up preview (remove newlines, collapse spaces)
		preview = strings.ReplaceAll(preview, "\n", " ")
		preview = strings.Join(strings.Fields(preview), " ")

		// Truncate preview to fit (rune-safe for Unicode)
		maxPreviewLen := 60
		if runes := []rune(preview); len(runes) > maxPreviewLen {
			preview = string(runes[:maxPreviewLen-3]) + "..."
		}

		header := fmt.Sprintf("%s thinking (%s tokens) ▶", thinkingIcon, tokenStr)
		if preview != "" {
			// Add preview in subtle style
			lines = append(lines, thinkingStyle.Render(header)+" "+styles.Subtle.Render(preview))
		} else {
			lines = append(lines, thinkingStyle.Render(header))
		}
	}

	// Store in cache (td-8910b218)
	p.setCachedRender(thinkingCacheID, maxWidth, expanded, strings.Join(lines, "\n"))
	return lines
}

// renderToolUseBlock renders a tool use block with its result (expand/collapse).
func (p *Plugin) renderToolUseBlock(block adapter.ContentBlock, maxWidth int) []string {
	var lines []string

	// Tool-specific icons for visual distinction
	icon := "⚙"
	toolName := block.ToolName
	switch strings.ToLower(toolName) {
	case "read":
		icon = "◉" // Filled circle for read
	case "edit", "str_replace_editor":
		icon = "◈" // Diamond for edit
	case "write":
		icon = "◇" // Empty diamond for write (new file)
	case "bash", "shell":
		icon = "$" // Shell prompt
	case "glob", "grep", "search":
		icon = "⊙" // Target/search symbol
	case "list", "ls":
		icon = "▤" // List symbol
	case "todoread", "todowrite":
		icon = "☐" // Checkbox for tasks
	}

	// Build tool header with icon and name
	toolHeader := icon + " " + toolName

	// Try to extract a meaningful command preview
	cmdPreview := extractToolCommand(block.ToolName, block.ToolInput, maxWidth-len(toolHeader)-5)
	if cmdPreview == "" {
		// Fall back to file_path extraction
		if filePath := extractFilePath(block.ToolInput); filePath != "" {
			cmdPreview = filePath
		}
	}
	if cmdPreview != "" {
		toolHeader += ": " + cmdPreview
	}

	if len(toolHeader) > maxWidth-2 {
		toolHeader = ui.TruncateString(toolHeader, maxWidth-2)
	}

	expanded := p.expandedToolResults[block.ToolUseID]

	// Style based on error state
	if block.IsError {
		// Red styling for errors with ✗ indicator
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F87171")) // Light red
		// Build error header without the original icon (avoid byte slicing Unicode)
		errorHeader := "✗ " + strings.TrimPrefix(toolHeader, icon+" ")
		lines = append(lines, errorStyle.Render(errorHeader))
	} else {
		lines = append(lines, styles.Code.Render(toolHeader))
	}

	// Show result if expanded or if there's an error
	if block.ToolOutput != "" && (expanded || block.IsError) {
		output := block.ToolOutput

		// Truncate before prettifying to prevent memory issues with large outputs
		const maxChars = 10000
		if len([]rune(output)) > maxChars {
			output = string([]rune(output)[:maxChars])
		}

		// Try to prettify JSON output
		output = prettifyJSON(output)

		maxOutputLines := 20
		outputLines := strings.Split(output, "\n")
		if len(outputLines) > maxOutputLines {
			outputLines = outputLines[:maxOutputLines]
			outputLines = append(outputLines, fmt.Sprintf("... (%d more lines)", len(strings.Split(output, "\n"))-maxOutputLines))
		}
		for _, line := range outputLines {
			if len(line) > maxWidth-4 {
				line = ui.TruncateString(line, maxWidth-4)
			}
			lines = append(lines, styles.Muted.Render("  "+line))
		}
	} else if block.ToolOutput != "" {
		// Collapsed: show first meaningful line of output as preview
		// Skip lines that are just JSON structural chars (not informative)
		outputLines := strings.Split(block.ToolOutput, "\n")
		preview := ""
		for _, line := range outputLines {
			trimmed := strings.TrimSpace(line)
			// Skip empty lines and single-char JSON structure
			if trimmed == "" || trimmed == "{" || trimmed == "[" || trimmed == "}" || trimmed == "]" {
				continue
			}
			preview = trimmed
			break
		}
		if preview != "" {
			// Show first meaningful line as preview (rune-safe for Unicode)
			if runes := []rune(preview); len(runes) > maxWidth-6 {
				preview = string(runes[:maxWidth-9]) + "..."
			}
			lines = append(lines, styles.Muted.Render("  → "+preview))
		}
	}

	return lines
}

// renderFilterMenu renders the filter selection menu.
func (p *Plugin) renderFilterMenu(height int) string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Filters"))
	sb.WriteString("                    ")
	sb.WriteString(styles.Muted.Render("[esc to cancel]"))
	sb.WriteString("\n")
	sb.WriteString(styles.Muted.Render(strings.Repeat("─", p.sidebarWidth-4)))
	sb.WriteString("\n\n")

	// Adapter filters
	adapterOptions := adapterFilterOptions(p.adapters)
	if len(adapterOptions) > 0 {
		sb.WriteString(styles.Subtitle.Render("Adapter:"))
		sb.WriteString("\n")
		for _, opt := range adapterOptions {
			checkbox := "[ ]"
			if p.filters.HasAdapter(opt.id) {
				checkbox = "[✓]"
			}
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", styles.Code.Render(opt.key), checkbox, opt.name))
		}
		sb.WriteString("\n")
	}

	// Model filters
	sb.WriteString(styles.Subtitle.Render("Model:"))
	sb.WriteString("\n")
	models := []struct {
		key   string
		name  string
		model string
	}{
		{"1", "Opus", "opus"},
		{"2", "Sonnet", "sonnet"},
		{"3", "Haiku", "haiku"},
	}
	for _, m := range models {
		checkbox := "[ ]"
		if p.filters.HasModel(m.model) {
			checkbox = "[✓]"
		}
		sb.WriteString(fmt.Sprintf("  %s %s %s\n", styles.Code.Render(m.key), checkbox, m.name))
	}
	sb.WriteString("\n")

	// Date filters
	sb.WriteString(styles.Subtitle.Render("Date:"))
	sb.WriteString("\n")
	dates := []struct {
		key    string
		name   string
		preset string
	}{
		{"t", "Today", "today"},
		{"y", "Yesterday", "yesterday"},
		{"w", "This Week", "week"},
	}
	for _, d := range dates {
		checkbox := "[ ]"
		if p.filters.DateRange.Preset == d.preset {
			checkbox = "[✓]"
		}
		sb.WriteString(fmt.Sprintf("  %s %s %s\n", styles.Code.Render(d.key), checkbox, d.name))
	}
	sb.WriteString("\n")

	// Active only
	activeCheck := "[ ]"
	if p.filters.ActiveOnly {
		activeCheck = "[✓]"
	}
	sb.WriteString(fmt.Sprintf("  %s %s Active only\n", styles.Code.Render("a"), activeCheck))
	sb.WriteString("\n")

	// Clear filters
	sb.WriteString(fmt.Sprintf("  %s Clear all filters\n", styles.Code.Render("x")))

	return sb.String()
}
