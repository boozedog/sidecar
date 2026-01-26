package tty

import (
	"strings"
	"testing"
)

func TestNewOutputBuffer(t *testing.T) {
	buf := NewOutputBuffer(100)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer, got %d lines", buf.Len())
	}
}

func TestOutputBuffer_Write(t *testing.T) {
	buf := NewOutputBuffer(100)
	buf.Write("line1\nline2\nline3")

	if buf.Len() != 3 {
		t.Errorf("expected 3 lines, got %d", buf.Len())
	}

	lines := buf.Lines()
	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestOutputBuffer_Update(t *testing.T) {
	buf := NewOutputBuffer(100)

	// First write should return true
	changed := buf.Update("hello\nworld")
	if !changed {
		t.Error("expected changed=true for initial write")
	}

	// Same content should return false
	changed = buf.Update("hello\nworld")
	if changed {
		t.Error("expected changed=false for same content")
	}

	// Different content should return true
	changed = buf.Update("hello\nuniverse")
	if !changed {
		t.Error("expected changed=true for different content")
	}
}

func TestOutputBuffer_Capacity(t *testing.T) {
	buf := NewOutputBuffer(3)

	// Write more lines than capacity
	buf.Write("line1\nline2\nline3\nline4\nline5")

	if buf.Len() != 3 {
		t.Errorf("expected 3 lines (capacity), got %d", buf.Len())
	}

	lines := buf.Lines()
	// Should keep most recent lines
	if lines[0] != "line3" || lines[1] != "line4" || lines[2] != "line5" {
		t.Errorf("expected most recent lines, got: %v", lines)
	}
}

func TestOutputBuffer_StripMouseSequences(t *testing.T) {
	buf := NewOutputBuffer(100)

	// Content with mouse escape sequences
	content := "hello\x1b[<65;83;33Mworld"
	buf.Write(content)

	// Mouse sequences should be stripped
	result := buf.String()
	if strings.Contains(result, "\x1b[<") {
		t.Error("expected mouse sequences to be stripped")
	}
	if !strings.Contains(result, "hello") || !strings.Contains(result, "world") {
		t.Error("expected content to be preserved")
	}
}

func TestOutputBuffer_StripTerminalModeSequences(t *testing.T) {
	buf := NewOutputBuffer(100)

	// Content with terminal mode sequences
	content := "hello\x1b[?2004hworld"
	buf.Write(content)

	result := buf.String()
	if strings.Contains(result, "\x1b[?2004h") {
		t.Error("expected terminal mode sequences to be stripped")
	}
}

func TestOutputBuffer_LinesRange(t *testing.T) {
	buf := NewOutputBuffer(100)
	buf.Write("line0\nline1\nline2\nline3\nline4")

	lines := buf.LinesRange(1, 3)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("unexpected lines: %v", lines)
	}
}

func TestOutputBuffer_Clear(t *testing.T) {
	buf := NewOutputBuffer(100)
	buf.Write("hello\nworld")

	if buf.Len() != 2 {
		t.Errorf("expected 2 lines before clear, got %d", buf.Len())
	}

	buf.Clear()

	if buf.Len() != 0 {
		t.Errorf("expected 0 lines after clear, got %d", buf.Len())
	}
}

func TestPartialMouseSeqRegex(t *testing.T) {
	tests := []struct {
		input string
		match bool
	}{
		{"[<65;83;33M", true},   // scroll down
		{"[<64;10;5M", true},    // scroll up
		{"[<0;50;20m", true},    // release
		{"hello", false},        // normal text
		{"[notmouse]", false},   // not a mouse sequence
		{"[<abc;def;ghiM", false}, // invalid format
	}

	for _, tt := range tests {
		if got := PartialMouseSeqRegex.MatchString(tt.input); got != tt.match {
			t.Errorf("PartialMouseSeqRegex.MatchString(%q) = %v, want %v", tt.input, got, tt.match)
		}
	}
}
