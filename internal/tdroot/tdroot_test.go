package tdroot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTDRoot_NoFile(t *testing.T) {
	// Create temp directory without .td-root
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	result := ResolveTDRoot(tmpDir)
	if result != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, result)
	}
}

func TestResolveTDRoot_ValidFile(t *testing.T) {
	// Create temp directory with .td-root pointing to another path
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetRoot := "/path/to/main/repo"
	tdRootPath := filepath.Join(tmpDir, TDRootFile)
	if err := os.WriteFile(tdRootPath, []byte(targetRoot+"\n"), 0644); err != nil {
		t.Fatalf("failed to write .td-root: %v", err)
	}

	result := ResolveTDRoot(tmpDir)
	if result != targetRoot {
		t.Errorf("expected %q, got %q", targetRoot, result)
	}
}

func TestResolveTDRoot_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write empty .td-root file
	tdRootPath := filepath.Join(tmpDir, TDRootFile)
	if err := os.WriteFile(tdRootPath, []byte("  \n"), 0644); err != nil {
		t.Fatalf("failed to write .td-root: %v", err)
	}

	result := ResolveTDRoot(tmpDir)
	if result != tmpDir {
		t.Errorf("expected %q (fallback to workDir), got %q", tmpDir, result)
	}
}

func TestResolveTDRoot_WhitespaceHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetRoot := "/path/to/main/repo"
	tdRootPath := filepath.Join(tmpDir, TDRootFile)
	// Write with extra whitespace and newlines
	if err := os.WriteFile(tdRootPath, []byte("  "+targetRoot+"  \n\n"), 0644); err != nil {
		t.Fatalf("failed to write .td-root: %v", err)
	}

	result := ResolveTDRoot(tmpDir)
	if result != targetRoot {
		t.Errorf("expected %q, got %q", targetRoot, result)
	}
}

func TestResolveDBPath_NoTDRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	expected := filepath.Join(tmpDir, TodosDir, DBFile)
	result := ResolveDBPath(tmpDir)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveDBPath_WithTDRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetRoot := "/path/to/main/repo"
	tdRootPath := filepath.Join(tmpDir, TDRootFile)
	if err := os.WriteFile(tdRootPath, []byte(targetRoot+"\n"), 0644); err != nil {
		t.Fatalf("failed to write .td-root: %v", err)
	}

	expected := filepath.Join(targetRoot, TodosDir, DBFile)
	result := ResolveDBPath(tmpDir)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCreateTDRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetRoot := "/path/to/main/repo"
	if err := CreateTDRoot(tmpDir, targetRoot); err != nil {
		t.Fatalf("CreateTDRoot failed: %v", err)
	}

	// Verify file was created with correct content
	tdRootPath := filepath.Join(tmpDir, TDRootFile)
	data, err := os.ReadFile(tdRootPath)
	if err != nil {
		t.Fatalf("failed to read .td-root: %v", err)
	}

	expected := targetRoot + "\n"
	if string(data) != expected {
		t.Errorf("expected content %q, got %q", expected, string(data))
	}
}

func TestCreateTDRoot_Overwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tdroot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial file
	if err := CreateTDRoot(tmpDir, "/old/path"); err != nil {
		t.Fatalf("first CreateTDRoot failed: %v", err)
	}

	// Overwrite with new path
	newTarget := "/new/path/to/repo"
	if err := CreateTDRoot(tmpDir, newTarget); err != nil {
		t.Fatalf("second CreateTDRoot failed: %v", err)
	}

	// Verify new content
	result := ResolveTDRoot(tmpDir)
	if result != newTarget {
		t.Errorf("expected %q, got %q", newTarget, result)
	}
}
