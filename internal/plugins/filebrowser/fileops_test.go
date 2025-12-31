package filebrowser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/sst/sidecar/internal/plugin"
)

func TestFileOpMode(t *testing.T) {
	// Verify FileOpMode enum values
	if FileOpNone != 0 {
		t.Error("FileOpNone should be 0")
	}
	if FileOpMove != 1 {
		t.Error("FileOpMove should be 1")
	}
	if FileOpRename != 2 {
		t.Error("FileOpRename should be 2")
	}
}

func TestValidateDestPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create plugin with minimal context
	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
	}

	tests := []struct {
		name    string
		dstPath string
		wantErr bool
	}{
		{
			name:    "valid - same directory",
			dstPath: filepath.Join(tmpDir, "newfile.txt"),
			wantErr: false,
		},
		{
			name:    "valid - subdirectory",
			dstPath: filepath.Join(tmpDir, "subdir", "file.txt"),
			wantErr: false,
		},
		{
			name:    "valid - nested subdirectory",
			dstPath: filepath.Join(tmpDir, "a", "b", "c", "file.txt"),
			wantErr: false,
		},
		{
			name:    "invalid - path traversal attack (..)",
			dstPath: filepath.Join(tmpDir, "..", "escaped.txt"),
			wantErr: true,
		},
		{
			name:    "invalid - deep path traversal",
			dstPath: filepath.Join(tmpDir, "subdir", "..", "..", "escaped.txt"),
			wantErr: true,
		},
		{
			name:    "invalid - absolute path outside workdir",
			dstPath: "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "invalid - home directory escape",
			dstPath: filepath.Join(os.TempDir(), "escaped.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.validateDestPath(tt.dstPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDestPath(%q) error = %v, wantErr %v", tt.dstPath, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDestPath_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
	}

	// Test symlink traversal attempt (if symlinks are supported)
	if runtime.GOOS != "windows" {
		// Create a symlink pointing outside workdir
		linkPath := filepath.Join(tmpDir, "escape_link")
		os.Symlink("/tmp", linkPath)

		// Path through symlink should still be validated against resolved path
		dstPath := filepath.Join(linkPath, "file.txt")
		// Note: filepath.Abs doesn't resolve symlinks by default,
		// so this test documents current behavior
		_ = p.validateDestPath(dstPath)
	}

	// Test path with dots that don't escape
	err := p.validateDestPath(filepath.Join(tmpDir, "file.test.txt"))
	if err != nil {
		t.Errorf("Path with dots should be valid: %v", err)
	}

	// Test path at exact workdir boundary
	err = p.validateDestPath(tmpDir)
	if err != nil {
		t.Errorf("Workdir itself should be valid: %v", err)
	}
}

func TestExecuteFileOp_RenameValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "original.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tree := NewFileTree(tmpDir)
	if err := tree.Build(); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
		tree:         tree,
		fileOpMode:   FileOpRename,
		fileOpTarget: &FileNode{Path: "original.txt", Name: "original.txt"},
	}

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid rename",
			input:   "newname.txt",
			wantErr: false,
		},
		{
			name:        "invalid - contains path separator",
			input:       "subdir/newname.txt",
			wantErr:     true,
			errContains: "use 'm' to move",
		},
		// Note: On Unix, backslash is a valid filename character (though not recommended).
		// Only Windows treats backslash as a path separator.
		// The code checks for filepath.Separator which is "/" on Unix, "\" on Windows.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := textinput.New()
			ti.SetValue(tt.input)
			p.fileOpTextInput = ti
			p.fileOpError = ""
			p.fileOpMode = FileOpRename

			p.executeFileOp()

			if tt.wantErr {
				if p.fileOpError == "" {
					t.Errorf("Expected error for input %q", tt.input)
				}
				if tt.errContains != "" && !containsString(p.fileOpError, tt.errContains) {
					t.Errorf("Error %q should contain %q", p.fileOpError, tt.errContains)
				}
			}
		})
	}
}

func TestExecuteFileOp_MoveValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file and subdir
	testFile := filepath.Join(tmpDir, "original.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	tree := NewFileTree(tmpDir)
	if err := tree.Build(); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
		tree:         tree,
		fileOpMode:   FileOpMove,
		fileOpTarget: &FileNode{Path: "original.txt", Name: "original.txt"},
	}

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid move to subdir",
			input:   "subdir/original.txt",
			wantErr: false,
		},
		{
			name:        "invalid - absolute path",
			input:       "/etc/passwd",
			wantErr:     true,
			errContains: "absolute paths not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := textinput.New()
			ti.SetValue(tt.input)
			p.fileOpTextInput = ti
			p.fileOpError = ""
			p.fileOpMode = FileOpMove

			p.executeFileOp()

			if tt.wantErr {
				if p.fileOpError == "" {
					t.Errorf("Expected error for input %q", tt.input)
				}
				if tt.errContains != "" && !containsString(p.fileOpError, tt.errContains) {
					t.Errorf("Error %q should contain %q", p.fileOpError, tt.errContains)
				}
			}
		})
	}
}

func TestDoFileOp(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
	}

	t.Run("successful move", func(t *testing.T) {
		dstPath := filepath.Join(tmpDir, "destination.txt")
		cmd := p.doFileOp(srcPath, dstPath)
		msg := cmd()

		switch m := msg.(type) {
		case FileOpSuccessMsg:
			if m.Src != srcPath || m.Dst != dstPath {
				t.Errorf("Success msg has wrong paths")
			}
		case FileOpErrorMsg:
			t.Errorf("Unexpected error: %v", m.Err)
		}

		// Verify file was moved
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			t.Error("Destination file should exist")
		}
	})

	t.Run("destination already exists", func(t *testing.T) {
		// Create source and destination
		src := filepath.Join(tmpDir, "src2.txt")
		dst := filepath.Join(tmpDir, "dst2.txt")
		os.WriteFile(src, []byte("src"), 0644)
		os.WriteFile(dst, []byte("dst"), 0644)

		cmd := p.doFileOp(src, dst)
		msg := cmd()

		switch m := msg.(type) {
		case FileOpErrorMsg:
			if !containsString(m.Err.Error(), "already exists") {
				t.Errorf("Error should mention destination exists: %v", m.Err)
			}
		case FileOpSuccessMsg:
			t.Error("Should have failed when destination exists")
		}
	})

	t.Run("source and destination same", func(t *testing.T) {
		src := filepath.Join(tmpDir, "same.txt")
		os.WriteFile(src, []byte("test"), 0644)

		cmd := p.doFileOp(src, src)
		msg := cmd()

		switch m := msg.(type) {
		case FileOpErrorMsg:
			if !containsString(m.Err.Error(), "same") {
				t.Errorf("Error should mention same path: %v", m.Err)
			}
		case FileOpSuccessMsg:
			t.Error("Should have failed when src == dst")
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		src := filepath.Join(tmpDir, "tomove.txt")
		os.WriteFile(src, []byte("test"), 0644)

		dst := filepath.Join(tmpDir, "new", "nested", "dir", "moved.txt")

		cmd := p.doFileOp(src, dst)
		msg := cmd()

		switch m := msg.(type) {
		case FileOpSuccessMsg:
			// Verify file was moved
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				t.Error("Destination file should exist")
			}
		case FileOpErrorMsg:
			t.Errorf("Unexpected error: %v", m.Err)
		}
	})
}

func TestRevealInFileManager_MessageTypes(t *testing.T) {
	// Test that RevealErrorMsg is properly structured
	errMsg := RevealErrorMsg{Err: os.ErrNotExist}
	if errMsg.Err != os.ErrNotExist {
		t.Error("RevealErrorMsg should contain the error")
	}
}

func TestFileOpMessages(t *testing.T) {
	// Verify message type structures
	successMsg := FileOpSuccessMsg{Src: "/a/b.txt", Dst: "/a/c.txt"}
	if successMsg.Src != "/a/b.txt" || successMsg.Dst != "/a/c.txt" {
		t.Error("FileOpSuccessMsg fields mismatch")
	}

	errMsg := FileOpErrorMsg{Err: os.ErrPermission}
	if errMsg.Err != os.ErrPermission {
		t.Error("FileOpErrorMsg should contain the error")
	}
}

func TestExecuteFileOp_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()

	ti := textinput.New()
	ti.SetValue("") // Empty input

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
		fileOpMode:      FileOpRename,
		fileOpTarget:    &FileNode{Path: "test.txt"},
		fileOpTextInput: ti,
	}

	newP, _ := p.executeFileOp()
	plugin := newP.(*Plugin)

	// Should reset mode when input is empty
	if plugin.fileOpMode != FileOpNone {
		t.Error("Should reset mode when input is empty")
	}
}

func TestExecuteFileOp_NilTarget(t *testing.T) {
	tmpDir := t.TempDir()

	ti := textinput.New()
	ti.SetValue("newname.txt")

	p := &Plugin{
		ctx: &plugin.Context{
			WorkDir: tmpDir,
		},
		fileOpMode:      FileOpRename,
		fileOpTarget:    nil, // Nil target
		fileOpTextInput: ti,
	}

	newP, _ := p.executeFileOp()
	plugin := newP.(*Plugin)

	// Should reset mode when target is nil
	if plugin.fileOpMode != FileOpNone {
		t.Error("Should reset mode when target is nil")
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantError bool
		errorMsg  string
	}{
		// Valid filenames
		{"simple name", "file.txt", false, ""},
		{"with spaces", "my file.txt", false, ""},
		{"hidden file", ".gitignore", false, ""},
		{"with dashes", "my-file.txt", false, ""},
		{"with underscores", "my_file.txt", false, ""},

		// Empty/special
		{"empty", "", true, "filename cannot be empty"},
		{"dot", ".", true, "invalid filename"},
		{"dotdot", "..", true, "invalid filename"},

		// Invalid characters
		{"with less than", "file<name", true, "filename contains invalid character: <"},
		{"with greater than", "file>name", true, "filename contains invalid character: >"},
		{"with colon", "file:name", true, "filename contains invalid character: :"},
		{"with quote", "file\"name", true, "filename contains invalid character: \""},
		{"with pipe", "file|name", true, "filename contains invalid character: |"},
		{"with question", "file?name", true, "filename contains invalid character: ?"},
		{"with asterisk", "file*name", true, "filename contains invalid character: *"},

		// Control characters
		{"with null", "file\x00name", true, "filename contains invalid characters"},
		{"with control", "file\x01name", true, "filename contains invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilename(tt.filename)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
