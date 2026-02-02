package ui

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestRenderScrollbar_SpacerWhenAllVisible(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   5,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  5,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if line != " " {
			t.Errorf("line %d: expected single space, got %q", i, line)
		}
	}
}

func TestRenderScrollbar_SpacerWhenEqual(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   10,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  5,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if line != " " {
			t.Errorf("line %d: expected single space, got %q", i, line)
		}
	}
}

func TestRenderScrollbar_ThumbAtTop(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  10,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Thumb should be at position 0 (top). Thumb size = (10*10)/100 = 1.
	// First line should contain the thumb char (┃), rest should be track (│).
	if !strings.Contains(lines[0], "┃") {
		t.Errorf("expected thumb at line 0, got %q", lines[0])
	}
	for i := 1; i < 10; i++ {
		if !strings.Contains(lines[i], "│") {
			t.Errorf("expected track at line %d, got %q", i, lines[i])
		}
	}
}

func TestRenderScrollbar_ThumbAtBottom(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 90, // TotalItems - VisibleItems
		VisibleItems: 10,
		TrackHeight:  10,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Thumb should be at last line.
	if !strings.Contains(lines[9], "┃") {
		t.Errorf("expected thumb at last line, got %q", lines[9])
	}
	for i := 0; i < 9; i++ {
		if !strings.Contains(lines[i], "│") {
			t.Errorf("expected track at line %d, got %q", i, lines[i])
		}
	}
}

func TestRenderScrollbar_ThumbAtMiddle(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 45,
		VisibleItems: 10,
		TrackHeight:  10,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// thumbSize = 1, thumbPos = 45 * 9 / 90 = 4 (middle-ish)
	thumbFound := -1
	for i, line := range lines {
		if strings.Contains(line, "┃") {
			thumbFound = i
			break
		}
	}
	if thumbFound < 1 || thumbFound > 8 {
		t.Errorf("expected thumb in middle range, found at %d", thumbFound)
	}
}

func TestRenderScrollbar_MinThumbSize(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   10000,
		ScrollOffset: 0,
		VisibleItems: 1,
		TrackHeight:  10,
	})
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// With 10000 items and 1 visible, thumb would be tiny but min is 1.
	thumbCount := 0
	for _, line := range lines {
		if strings.Contains(line, "┃") {
			thumbCount++
		}
	}
	if thumbCount < 1 {
		t.Error("expected at least 1 thumb line")
	}
}

func TestRenderScrollbar_ExactLineCount(t *testing.T) {
	for _, h := range []int{1, 5, 20, 50} {
		result := RenderScrollbar(ScrollbarParams{
			TotalItems:   100,
			ScrollOffset: 0,
			VisibleItems: 10,
			TrackHeight:  h,
		})
		lines := strings.Split(result, "\n")
		if len(lines) != h {
			t.Errorf("TrackHeight=%d: expected %d lines, got %d", h, h, len(lines))
		}
	}
}

func TestRenderScrollbar_SingleColumnWide(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  10,
	})
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		w := runewidth.StringWidth(stripAnsi(line))
		if w != 1 {
			t.Errorf("line %d: expected width 1, got %d (line=%q)", i, w, line)
		}
	}
}

func TestRenderScrollbar_ZeroTrackHeight(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  0,
	})
	if result != "" {
		t.Errorf("expected empty string for TrackHeight=0, got %q", result)
	}
}

func TestRenderScrollbar_NegativeTrackHeight(t *testing.T) {
	result := RenderScrollbar(ScrollbarParams{
		TotalItems:   100,
		ScrollOffset: 0,
		VisibleItems: 10,
		TrackHeight:  -5,
	})
	if result != "" {
		t.Errorf("expected empty string for negative TrackHeight, got %q", result)
	}
}

// stripAnsi removes ANSI escape sequences for width measurement.
func stripAnsi(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
