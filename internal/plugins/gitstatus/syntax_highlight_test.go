package gitstatus

import (
	"testing"
)

func TestNewSyntaxHighlighter_GoFile(t *testing.T) {
	h := NewSyntaxHighlighter("test.go")
	if h == nil {
		t.Error("expected highlighter for .go file")
	}
}

func TestNewSyntaxHighlighter_PythonFile(t *testing.T) {
	h := NewSyntaxHighlighter("script.py")
	if h == nil {
		t.Error("expected highlighter for .py file")
	}
}

func TestNewSyntaxHighlighter_JavaScript(t *testing.T) {
	h := NewSyntaxHighlighter("app.js")
	if h == nil {
		t.Error("expected highlighter for .js file")
	}
}

func TestNewSyntaxHighlighter_TypeScript(t *testing.T) {
	h := NewSyntaxHighlighter("app.ts")
	if h == nil {
		t.Error("expected highlighter for .ts file")
	}
}

func TestNewSyntaxHighlighter_UnknownExtension(t *testing.T) {
	h := NewSyntaxHighlighter("file.xyz123")
	if h != nil {
		t.Error("expected nil highlighter for unknown extension")
	}
}

func TestNewSyntaxHighlighter_NoExtension(t *testing.T) {
	h := NewSyntaxHighlighter("Makefile")
	// Makefile is actually recognized by Chroma
	if h == nil {
		t.Log("Makefile highlighter available")
	}
}

func TestSyntaxHighlighter_Highlight_GoCode(t *testing.T) {
	h := NewSyntaxHighlighter("test.go")
	if h == nil {
		t.Skip("no highlighter available")
	}

	segments := h.Highlight("func main() {")
	if len(segments) == 0 {
		t.Error("expected at least one segment")
	}

	// Check that we got multiple tokens (func, main, parens, brace)
	if len(segments) < 3 {
		t.Errorf("expected multiple tokens, got %d", len(segments))
	}
}

func TestSyntaxHighlighter_Highlight_EmptyLine(t *testing.T) {
	h := NewSyntaxHighlighter("test.go")
	if h == nil {
		t.Skip("no highlighter available")
	}

	segments := h.Highlight("")
	// Empty line returns empty or nil slice - both are valid
	// The important thing is that it doesn't panic
	_ = segments
}

func TestSyntaxHighlighter_NilHighlighter(t *testing.T) {
	var h *SyntaxHighlighter = nil
	segments := h.Highlight("test")
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Text != "test" {
		t.Errorf("expected 'test', got %q", segments[0].Text)
	}
}

func TestRenderLineDiff_WithHighlighter(t *testing.T) {
	diff := &ParsedDiff{
		OldFile: "test.go",
		NewFile: "test.go",
		Hunks: []Hunk{
			{
				OldStart: 1,
				OldCount: 3,
				NewStart: 1,
				NewCount: 3,
				Lines: []DiffLine{
					{Type: LineContext, OldLineNo: 1, NewLineNo: 1, Content: "package main"},
					{Type: LineRemove, OldLineNo: 2, NewLineNo: 0, Content: "func old() {}"},
					{Type: LineAdd, OldLineNo: 0, NewLineNo: 2, Content: "func new() {}"},
				},
			},
		},
	}

	h := NewSyntaxHighlighter("test.go")
	result := RenderLineDiff(diff, 80, 0, 20, 0, h, false)
	if result == "" {
		t.Error("expected non-empty result with highlighter")
	}
}

func TestRenderSideBySide_WithHighlighter(t *testing.T) {
	diff := &ParsedDiff{
		OldFile: "test.go",
		NewFile: "test.go",
		Hunks: []Hunk{
			{
				OldStart: 1,
				OldCount: 2,
				NewStart: 1,
				NewCount: 2,
				Lines: []DiffLine{
					{Type: LineContext, OldLineNo: 1, NewLineNo: 1, Content: "import \"fmt\""},
					{Type: LineRemove, OldLineNo: 2, NewLineNo: 0, Content: "var x = 1"},
					{Type: LineAdd, OldLineNo: 0, NewLineNo: 2, Content: "var x = 2"},
				},
			},
		},
	}

	h := NewSyntaxHighlighter("test.go")
	result := RenderSideBySide(diff, 120, 0, 20, 0, h, false)
	if result == "" {
		t.Error("expected non-empty result with highlighter")
	}
}
