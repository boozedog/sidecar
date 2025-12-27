package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IntroModel handles the intro animation state.
type IntroModel struct {
	Active    bool
	StartTime time.Time
	Letters   []*IntroLetter
	Done      bool // Set to true when animation is finished
}

type IntroLetter struct {
	Char     rune
	TargetX  float64
	CurrentX float64

	// Color interpolation
	StartColor   RGB
	EndColor     RGB
	CurrentColor RGB

	Delay time.Duration
}

type RGB struct {
	R, G, B float64
}

func hexToRGB(hex string) RGB {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return RGB{float64(r), float64(g), float64(b)}
}

func (c RGB) toLipgloss() lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", int(c.R), int(c.G), int(c.B)))
}

func NewIntroModel() IntroModel {
	text := "Sidecar"
	letters := make([]*IntroLetter, len(text))

	// Gradient endpoints for the final state
	// Orange: #F59E0B (245, 158, 11)
	// Dark Yellow: #D97706 (217, 119, 6) - slightly darker/more orange-yellow
	startGradient := hexToRGB("#F59E0B")
	endGradient := hexToRGB("#D97706")

	// Random distinct start colors
	startColors := []string{
		"#EF4444", // Red
		"#3B82F6", // Blue
		"#10B981", // Green
		"#8B5CF6", // Purple
		"#EC4899", // Pink
		"#06B6D4", // Cyan
		"#F97316", // Orange
	}

	for i, char := range text {
		// Calculate target color for this letter in the gradient
		t := float64(i) / float64(len(text)-1)
		targetColor := RGB{
			R: startGradient.R + t*(endGradient.R-startGradient.R),
			G: startGradient.G + t*(endGradient.G-startGradient.G),
			B: startGradient.B + t*(endGradient.B-startGradient.B),
		}

		letters[i] = &IntroLetter{
			Char:         char,
			CurrentX:     -20.0 - float64(i)*10.0, // Start further left and more spaced out
			TargetX:      float64(i),              // Target is adjacent index
			StartColor:   hexToRGB(startColors[i%len(startColors)]),
			EndColor:     targetColor,
			CurrentColor: hexToRGB(startColors[i%len(startColors)]),
			Delay:        time.Duration(i) * 150 * time.Millisecond,
		}
	}

	return IntroModel{
		Active:  true,
		Letters: letters,
	}
}

// Update progresses the animation
func (m *IntroModel) Update(dt time.Duration) {
	if !m.Active {
		return
	}

	allSettled := true

	for _, l := range m.Letters {
		// Check delay
		if m.StartTime.IsZero() {
			m.StartTime = time.Now()
		}
		elapsed := time.Since(m.StartTime)
		if elapsed < l.Delay {
			allSettled = false
			continue
		}

		// Animation logic (Spring-like or EaseOut)
		// Move towards TargetX
		speed := 25.0 // pixels per second
		dist := l.TargetX - l.CurrentX

		// Simple ease-out
		move := dist * 8.0 * dt.Seconds()
		if move > 0 && move < speed*dt.Seconds() {
			move = speed * dt.Seconds()
		}
		if move > dist {
			move = dist
		}

		l.CurrentX += move

		// Color interpolation
		// Interpolate towards EndColor
		colorSpeed := 3.0 * dt.Seconds()
		l.CurrentColor.R += (l.EndColor.R - l.CurrentColor.R) * colorSpeed
		l.CurrentColor.G += (l.EndColor.G - l.CurrentColor.G) * colorSpeed
		l.CurrentColor.B += (l.EndColor.B - l.CurrentColor.B) * colorSpeed

		// Check if settled
		if math.Abs(l.TargetX-l.CurrentX) > 0.1 ||
			math.Abs(l.EndColor.R-l.CurrentColor.R) > 1.0 {
			allSettled = false
		}
	}

	if allSettled {
		m.Done = true
	}
}

func (m IntroModel) View() string {
	if !m.Active {
		return ""
	}

	// We need to render the string "Sidecar" (length 7)
	// We'll create a buffer large enough to hold the text at target positions.
	// Since TargetX goes from 0 to 6, a buffer of size 7 is sufficient for the final state.
	// However, during animation, letters might be at negative positions (hidden)
	// or between positions. We'll map to the nearest integer index.

	length := len(m.Letters)
	buf := make([]string, length)
	for i := range buf {
		buf[i] = " "
	}

	for _, l := range m.Letters {
		x := int(math.Round(l.CurrentX))
		if x >= 0 && x < length {
			style := lipgloss.NewStyle().Foreground(l.CurrentColor.toLipgloss()).Bold(true)
			buf[x] = style.Render(string(l.Char))
		}
	}

	return strings.Join(buf, "")
}

// IntroTickMsg is sent to update the animation frame.
type IntroTickMsg time.Time

func IntroTick() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return IntroTickMsg(t)
	})
}
