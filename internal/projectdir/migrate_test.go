package projectdir

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marcus/sidecar/internal/config"
)

// setupMigrateTestConfig sets up an isolated config path for testing.
func setupMigrateTestConfig(t *testing.T) string {
	t.Helper()
	configDir := t.TempDir()
	config.SetTestConfigPath(filepath.Join(configDir, "config.json"))
	t.Cleanup(config.ResetTestConfigPath)
	return configDir
}

func TestMigrate_MovesLegacyFiles(t *testing.T) {
	configDir := setupMigrateTestConfig(t)

	projectRoot := t.TempDir()

	// Create legacy .sidecar/ directory with files
	sidecarDir := filepath.Join(projectRoot, ".sidecar")
	if err := os.MkdirAll(sidecarDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "shells.json"), []byte(`{"shells":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "config.json"), []byte(`{"prompts":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	// Create transient files that should be deleted
	if err := os.WriteFile(filepath.Join(sidecarDir, "shells.json.lock"), []byte("lock"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sidecarDir, "shells.json.tmp"), []byte("tmp"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create legacy .td-root
	if err := os.WriteFile(filepath.Join(projectRoot, ".td-root"), []byte("/some/root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run migration
	base := filepath.Dir(config.ConfigPath())
	if err := migrateWithBase(base, projectRoot); err != nil {
		t.Fatalf("migrateWithBase: %v", err)
	}

	// Verify centralized project dir was created
	projDir, err := resolveWithBase(configDir, projectRoot)
	if err != nil {
		t.Fatalf("resolveWithBase: %v", err)
	}

	// Check .sidecar/shells.json was moved
	assertFileContent(t, filepath.Join(projDir, "shells.json"), `{"shells":[]}`)
	assertFileNotExists(t, filepath.Join(sidecarDir, "shells.json"))

	// Check .sidecar/config.json was moved
	assertFileContent(t, filepath.Join(projDir, "config.json"), `{"prompts":[]}`)
	assertFileNotExists(t, filepath.Join(sidecarDir, "config.json"))

	// Check transient files were removed
	assertFileNotExists(t, filepath.Join(sidecarDir, "shells.json.lock"))
	assertFileNotExists(t, filepath.Join(sidecarDir, "shells.json.tmp"))

	// Check .sidecar directory was removed (now empty)
	assertFileNotExists(t, sidecarDir)

	// Check .td-root was moved
	assertFileContent(t, filepath.Join(projDir, "td-root"), "/some/root\n")
	assertFileNotExists(t, filepath.Join(projectRoot, ".td-root"))
}

func TestMigrate_NoopWhenNoLegacyFiles(t *testing.T) {
	setupMigrateTestConfig(t)

	projectRoot := t.TempDir()

	// No legacy files exist — migration should be a no-op
	base := filepath.Dir(config.ConfigPath())
	if err := migrateWithBase(base, projectRoot); err != nil {
		t.Fatalf("migrateWithBase: %v", err)
	}

	// Verify no centralized project dir was created (resolveWithBase would
	// create one, so we check the projects directory directly)
	projectsDir := filepath.Join(base, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		// Directory doesn't exist — that's fine, no migration happened
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no project dirs, got %d entries", len(entries))
	}
}

func TestMigrate_PartialLegacyFiles(t *testing.T) {
	configDir := setupMigrateTestConfig(t)

	projectRoot := t.TempDir()

	// Create only .td-root (partial legacy)
	if err := os.WriteFile(filepath.Join(projectRoot, ".td-root"), []byte("/partial/root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run migration
	base := filepath.Dir(config.ConfigPath())
	if err := migrateWithBase(base, projectRoot); err != nil {
		t.Fatalf("migrateWithBase: %v", err)
	}

	// Verify centralized project dir
	projDir, err := resolveWithBase(configDir, projectRoot)
	if err != nil {
		t.Fatalf("resolveWithBase: %v", err)
	}

	// Check .td-root was moved
	assertFileContent(t, filepath.Join(projDir, "td-root"), "/partial/root\n")
	assertFileNotExists(t, filepath.Join(projectRoot, ".td-root"))

	// Files that didn't exist should not have been created
	assertFileNotExists(t, filepath.Join(projDir, "shells.json"))
	assertFileNotExists(t, filepath.Join(projDir, "config.json"))
}

// assertFileContent reads a file and asserts its content matches expected.
func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("expected file %s to exist: %v", path, err)
		return
	}
	if string(data) != expected {
		t.Errorf("file %s content = %q, want %q", path, string(data), expected)
	}
}

// assertFileNotExists asserts that a path does not exist.
func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to not exist, but it does (err=%v)", path, err)
	}
}
