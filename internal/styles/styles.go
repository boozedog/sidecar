package styles

import "github.com/charmbracelet/lipgloss"

// Color palette - default dark theme
var (
	// Primary colors
	Primary   = lipgloss.Color("#7C3AED") // Purple
	Secondary = lipgloss.Color("#3B82F6") // Blue
	Accent    = lipgloss.Color("#F59E0B") // Amber

	// Status colors
	Success = lipgloss.Color("#10B981") // Green
	Warning = lipgloss.Color("#F59E0B") // Amber
	Error   = lipgloss.Color("#EF4444") // Red
	Info    = lipgloss.Color("#3B82F6") // Blue

	// Text colors
	TextPrimary   = lipgloss.Color("#F9FAFB")
	TextSecondary = lipgloss.Color("#9CA3AF")
	TextMuted     = lipgloss.Color("#6B7280")
	TextSubtle    = lipgloss.Color("#4B5563")

	// Background colors
	BgPrimary   = lipgloss.Color("#111827")
	BgSecondary = lipgloss.Color("#1F2937")
	BgTertiary  = lipgloss.Color("#374151")
	BgOverlay   = lipgloss.Color("#00000080")

	// Border colors
	BorderNormal = lipgloss.Color("#374151")
	BorderActive = lipgloss.Color("#7C3AED")
	BorderMuted  = lipgloss.Color("#1F2937")
)

// Panel styles
var (
	// Active panel with highlighted border
	PanelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderActive).
			Padding(0, 1)

	// Inactive panel with subtle border
	PanelInactive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderNormal).
			Padding(0, 1)

	// Panel header
	PanelHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextPrimary).
			MarginBottom(1)

	// Panel with no border
	PanelNoBorder = lipgloss.NewStyle().
			Padding(0, 1)
)

// Text styles
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(TextPrimary)

	Subtitle = lipgloss.NewStyle().
			Foreground(TextSecondary) // Matches inactive tab text for consistency

	Body = lipgloss.NewStyle().
		Foreground(TextPrimary)

	Muted = lipgloss.NewStyle().
		Foreground(TextMuted)

	Subtle = lipgloss.NewStyle().
		Foreground(TextSubtle)

	Code = lipgloss.NewStyle().
		Foreground(Accent)

	KeyHint = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(BgTertiary).
		Padding(0, 1)

	Logo = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)
)

// Status indicator styles
var (
	StatusStaged = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	StatusModified = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	// Toast styles for status messages
	ToastSuccess = lipgloss.NewStyle().
			Background(Success).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	ToastError = lipgloss.NewStyle().
			Background(Error).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	StatusUntracked = lipgloss.NewStyle().
			Foreground(TextMuted)

	StatusDeleted = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	StatusInProgress = lipgloss.NewStyle().
				Foreground(Info).
				Bold(true)

	StatusCompleted = lipgloss.NewStyle().
			Foreground(Success)

	StatusBlocked = lipgloss.NewStyle().
			Foreground(Error)

	StatusPending = lipgloss.NewStyle().
			Foreground(TextMuted)
)

// List item styles
var (
	ListItemNormal = lipgloss.NewStyle().
			Foreground(TextPrimary)

	ListItemSelected = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(BgTertiary)

	ListItemFocused = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(Primary)

	ListCursor = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)
)

// Bar element styles (shared by header/footer)
var (
	BarTitle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Bold(true)

	BarText = lipgloss.NewStyle().
		Foreground(TextMuted)

	BarChip = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(BgTertiary).
		Padding(0, 1)

	BarChipActive = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(Primary).
			Padding(0, 1).
			Bold(true)
)

// Tab bar styles (using bar element primitives)
var (
	TabActive = BarChipActive.Padding(0, 2)

	// TabInactive uses TextSecondary for better contrast (vs TextMuted)
	TabInactive = lipgloss.NewStyle().
			Foreground(TextSecondary).
			Background(BgTertiary).
			Padding(0, 2)
)

// Diff line styles
var (
	DiffAdd = lipgloss.NewStyle().
		Foreground(Success)

	DiffRemove = lipgloss.NewStyle().
			Foreground(Error)

	DiffContext = lipgloss.NewStyle().
			Foreground(TextMuted)

	DiffHeader = lipgloss.NewStyle().
			Foreground(Info).
			Bold(true)

	// Subtle diff backgrounds for syntax-highlighted lines
	DiffAddBg    = lipgloss.Color("#0D2818") // Very subtle dark green
	DiffRemoveBg = lipgloss.Color("#2D1A1A") // Very subtle dark red
)

// File browser styles
var (
	// Directory names - bold blue
	FileBrowserDir = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true)

	// Regular file names
	FileBrowserFile = lipgloss.NewStyle().
			Foreground(TextPrimary)

	// Gitignored files - muted/dimmed
	FileBrowserIgnored = lipgloss.NewStyle().
				Foreground(TextSubtle)

	// Line numbers in preview
	FileBrowserLineNumber = lipgloss.NewStyle().
				Foreground(TextMuted).
				Width(5).
				AlignHorizontal(lipgloss.Right)

	// Tree icons (>, +)
	FileBrowserIcon = lipgloss.NewStyle().
			Foreground(TextMuted)

	// Content search match highlighting
	SearchMatch = lipgloss.NewStyle().
			Background(Warning) // Yellow background for all matches

	SearchMatchCurrent = lipgloss.NewStyle().
				Background(Primary). // Purple background for current match
				Foreground(TextPrimary)

	// Fuzzy match character highlighting (bold in result list)
	FuzzyMatchChar = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Quick open result row (normal)
	QuickOpenItem = lipgloss.NewStyle().
			Foreground(TextPrimary)

	// Quick open result row (selected)
	QuickOpenItemSelected = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(BgTertiary)

	// Palette entry styles (reusable for modals)
	PaletteEntry = lipgloss.NewStyle().
			Foreground(TextPrimary)

	PaletteEntrySelected = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(BgTertiary)

	PaletteKey = lipgloss.NewStyle().
			Foreground(TextMuted).
			Background(BgTertiary).
			Padding(0, 1)

	// Text selection for preview pane drag selection
	TextSelection = lipgloss.NewStyle().
			Background(BgTertiary).
			Foreground(TextPrimary)
)

// Footer and header
var (
	Footer = lipgloss.NewStyle().
		Foreground(TextMuted).
		Background(BgSecondary)

	Header = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Background(BgSecondary).
		Bold(true)
)

// Modal styles
var (
	ModalOverlay = lipgloss.NewStyle().
			Background(BgOverlay)

	ModalBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Background(BgSecondary).
			Padding(1, 2)

	ModalTitle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Bold(true).
			MarginBottom(1)
)

// Button styles
var (
	Button = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Background(BgTertiary).
		Padding(0, 2)

	ButtonFocused = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Background(Primary).
			Padding(0, 2).
			Bold(true)
)

// Theme represents a color theme configuration
type Theme struct {
	Name      string
	Colors    ColorPalette
	Overrides map[string]string
}

// ColorPalette holds all theme colors
type ColorPalette struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
}

// DefaultTheme returns the default dark theme
func DefaultTheme() Theme {
	return Theme{
		Name: "default",
		Colors: ColorPalette{
			Primary:   Primary,
			Secondary: Secondary,
			Accent:    Accent,
			Success:   Success,
			Warning:   Warning,
			Error:     Error,
		},
		Overrides: make(map[string]string),
	}
}

// LoadTheme loads a theme by name with optional overrides
func LoadTheme(name string, overrides map[string]string) Theme {
	theme := DefaultTheme()
	theme.Name = name
	if overrides != nil {
		theme.Overrides = overrides
	}
	// Future: Load theme from config file by name
	return theme
}
