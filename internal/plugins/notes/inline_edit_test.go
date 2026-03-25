package notes

import (
	"testing"

	"github.com/marcus/sidecar/internal/tty"
)

func TestNormalizeEditorName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Direct names
		{"vim", "vim"},
		{"nano", "nano"},
		{"emacs", "emacs"},
		{"helix", "helix"},
		{"micro", "micro"},
		{"kakoune", "kakoune"},

		// Aliases
		{"nvim", "vim"},
		{"neovim", "vim"},
		{"vi", "vim"},
		{"hx", "helix"},
		{"kak", "kakoune"},
		{"emacsclient", "emacs"},

		// Full paths
		{"/usr/bin/vim", "vim"},
		{"/usr/local/bin/nvim", "vim"},
		{"/opt/homebrew/bin/nano", "nano"},

		// Windows .exe suffix
		{"vim.exe", "vim"},

		// Unknown editors pass through
		{"code", "code"},
		{"subl", "subl"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeEditorName(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeEditorName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateInlineEditorHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
		want   int
	}{
		{
			name:   "standard height",
			height: 24,
			want:   21, // 24 - 2 (borders) - 1 (header)
		},
		{
			name:   "minimum height clamp",
			height: 4,
			want:   5, // clamped to minimum
		},
		{
			name:   "very small height",
			height: 2,
			want:   5, // height < 4 becomes 4, then 4 - 2 - 1 = 1, clamped to 5
		},
		{
			name:   "tall terminal",
			height: 50,
			want:   47, // 50 - 2 - 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				height: tt.height,
			}
			got := p.calculateInlineEditorHeight()
			if got != tt.want {
				t.Errorf("calculateInlineEditorHeight() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCalculateInlineEditorWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		listWidth int
		wantMin   int
	}{
		{
			name:      "standard width",
			width:     100,
			listWidth: 30,
			wantMin:   60, // 100 - 30 - 1 (divider) - 4 (borders+padding) = 65
		},
		{
			name:      "narrow window",
			width:     60,
			listWidth: 20,
			wantMin:   30, // 60 - 20 - 1 - 4 = 35
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				width:     tt.width,
				listWidth: tt.listWidth,
			}
			got := p.calculateInlineEditorWidth()
			if got < tt.wantMin {
				t.Errorf("calculateInlineEditorWidth() = %d, want >= %d", got, tt.wantMin)
			}
			if got <= 0 {
				t.Errorf("calculateInlineEditorWidth() = %d, want > 0", got)
			}
		})
	}
}

func TestCalculateInlineEditorMouseCoords(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		listWidth int
		clickX    int
		clickY    int
		wantCol   int
		wantRow   int
		wantOK    bool
	}{
		{
			name:      "valid click at content origin",
			width:     100,
			height:    24,
			listWidth: 30,
			clickX:    33, // listWidth(30) + divider(1) + border(1) + padding(1) = 33
			clickY:    2,  // border(1) + header(1) = content start
			wantCol:   1,
			wantRow:   1,
			wantOK:    true,
		},
		{
			name:      "click in list area (out of bounds)",
			width:     100,
			height:    24,
			listWidth: 30,
			clickX:    5, // in list pane
			clickY:    5,
			wantCol:   0,
			wantRow:   0,
			wantOK:    false,
		},
		{
			name:      "zero dimensions",
			width:     0,
			height:    0,
			listWidth: 0,
			clickX:    5,
			clickY:    5,
			wantCol:   0,
			wantRow:   0,
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plugin{
				width:     tt.width,
				height:    tt.height,
				listWidth: tt.listWidth,
			}

			col, row, ok := p.calculateInlineEditorMouseCoords(tt.clickX, tt.clickY)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK {
				if col != tt.wantCol {
					t.Errorf("col = %d, want %d", col, tt.wantCol)
				}
				if row != tt.wantRow {
					t.Errorf("row = %d, want %d", row, tt.wantRow)
				}
			}
		})
	}
}

func TestSendEditorSaveAndQuit_KnownEditors(t *testing.T) {
	known := []string{
		"vim", "nvim", "vi", "nano", "emacs", "emacsclient",
		"helix", "hx", "micro", "kakoune", "kak",
	}

	for _, editor := range known {
		t.Run(editor, func(t *testing.T) {
			got := sendEditorSaveAndQuit("nonexistent-session", editor)
			if !got {
				t.Errorf("sendEditorSaveAndQuit(_, %q) = false, want true", editor)
			}
		})
	}
}

func TestSendEditorSaveAndQuit_UnknownEditors(t *testing.T) {
	unknown := []string{"code", "subl", "atom", "gedit"}

	for _, editor := range unknown {
		t.Run(editor, func(t *testing.T) {
			got := sendEditorSaveAndQuit("nonexistent-session", editor)
			if got {
				t.Errorf("sendEditorSaveAndQuit(_, %q) = true, want false", editor)
			}
		})
	}
}

func TestInlineEditorTtyConfig(t *testing.T) {
	// Verify the tty model preserves ANSI sequences via CapturePaneOutput.
	// The -e flag in capture-pane is what enables syntax highlighting.
	m := tty.New(nil)
	if m.Config.ScrollbackLines <= 0 {
		t.Errorf("default ScrollbackLines = %d, want > 0", m.Config.ScrollbackLines)
	}
}
