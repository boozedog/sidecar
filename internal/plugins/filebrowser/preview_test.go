package filebrowser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"text", []byte("hello world"), false},
		{"binary with null", []byte("hello\x00world"), true},
		{"empty", []byte{}, false},
		{"binary at start", []byte{0, 1, 2, 3}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinary(tt.data)
			if result != tt.expected {
				t.Errorf("isBinary(%v) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestLoadPreview(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testContent := "line 1\nline 2\nline 3"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Load preview
	cmd := LoadPreview(tmpDir, "test.txt", 0)
	msg := cmd()

	result, ok := msg.(PreviewLoadedMsg)
	if !ok {
		t.Fatalf("expected PreviewLoadedMsg, got %T", msg)
	}

	if result.Path != "test.txt" {
		t.Errorf("path = %q, want %q", result.Path, "test.txt")
	}

	if result.Result.Error != nil {
		t.Errorf("unexpected error: %v", result.Result.Error)
	}

	if result.Result.IsBinary {
		t.Error("expected non-binary file")
	}

	if len(result.Result.Lines) != 3 {
		t.Errorf("lines = %d, want 3", len(result.Result.Lines))
	}
}

func TestLoadPreview_Binary(t *testing.T) {
	tmpDir := t.TempDir()

	// Create binary file (use .bin extension to avoid image detection)
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00} // Binary data with null byte
	testFile := filepath.Join(tmpDir, "data.bin")
	if err := os.WriteFile(testFile, binaryData, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := LoadPreview(tmpDir, "data.bin", 0)
	msg := cmd()

	result := msg.(PreviewLoadedMsg)

	if !result.Result.IsBinary {
		t.Error("expected binary file")
	}
}

func TestLoadPreview_Image(t *testing.T) {
	tmpDir := t.TempDir()

	// Create image file (PNG-like data)
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00}
	testFile := filepath.Join(tmpDir, "image.png")
	if err := os.WriteFile(testFile, imageData, 0644); err != nil {
		t.Fatal(err)
	}

	cmd := LoadPreview(tmpDir, "image.png", 0)
	msg := cmd()

	result := msg.(PreviewLoadedMsg)

	if !result.Result.IsImage {
		t.Error("expected image file")
	}
	if result.Result.IsBinary {
		t.Error("image files should not be marked as binary")
	}
}

func TestLoadPreview_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create large file (> maxPreviewSize)
	largeContent := strings.Repeat("x", maxPreviewSize+1000)
	testFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(testFile, []byte(largeContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := LoadPreview(tmpDir, "large.txt", 0)
	msg := cmd()

	result := msg.(PreviewLoadedMsg)

	if !result.Result.IsTruncated {
		t.Error("expected truncated file")
	}
}

func TestLoadPreview_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := LoadPreview(tmpDir, "nonexistent.txt", 0)
	msg := cmd()

	result := msg.(PreviewLoadedMsg)

	if result.Result.Error == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestHighlight(t *testing.T) {
	content := `package main

func main() {
	println("hello")
}`

	result, err := Highlight(content, ".go", "monokai")
	if err != nil {
		t.Fatalf("Highlight failed: %v", err)
	}

	// Result should contain ANSI escape codes
	if !strings.Contains(result, "\x1b[") {
		t.Error("expected ANSI escape codes in highlighted output")
	}

	// Result should contain the original content
	if !strings.Contains(result, "main") {
		t.Error("expected 'main' in highlighted output")
	}
}

func TestLoadPreview_WithHighlighting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go file
	goContent := `package test

func Hello() string {
	return "world"
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := LoadPreview(tmpDir, "test.go", 0)
	msg := cmd()

	result := msg.(PreviewLoadedMsg)

	if result.Result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Result.Error)
	}

	if len(result.Result.HighlightedLines) == 0 {
		t.Error("expected highlighted lines")
	}

	// Highlighted lines should contain ANSI codes
	if len(result.Result.HighlightedLines) > 0 {
		firstLine := result.Result.HighlightedLines[0]
		if !strings.Contains(firstLine, "\x1b[") {
			t.Error("expected ANSI escape codes in first highlighted line")
		}
	}
}
