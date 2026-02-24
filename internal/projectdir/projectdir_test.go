package projectdir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWithBase_NewProject(t *testing.T) {
	base := t.TempDir()
	projectRoot := "/Users/alice/Projects/myapp"

	dir, err := resolveWithBase(base, projectRoot)
	if err != nil {
		t.Fatalf("resolveWithBase: %v", err)
	}

	// Directory should exist.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("expected directory to exist: %s", dir)
	}

	// Slug should be "myapp".
	expectedSlug := "myapp"
	if filepath.Base(dir) != expectedSlug {
		t.Errorf("directory slug = %q, want %q", filepath.Base(dir), expectedSlug)
	}

	// meta.json should contain the project path.
	metaPath := filepath.Join(dir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading meta.json: %v", err)
	}

	var meta projectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing meta.json: %v", err)
	}
	if meta.Path != projectRoot {
		t.Errorf("meta.Path = %q, want %q", meta.Path, projectRoot)
	}
}

func TestResolveWithBase_ExistingProject(t *testing.T) {
	base := t.TempDir()
	projectRoot := "/Users/bob/code/webapp"

	// First resolve creates the directory.
	dir1, err := resolveWithBase(base, projectRoot)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	// Second resolve should return the same directory.
	dir2, err := resolveWithBase(base, projectRoot)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	if dir1 != dir2 {
		t.Errorf("second resolve returned %q, want %q", dir2, dir1)
	}
}

func TestResolveWithBase_SlugCollision(t *testing.T) {
	base := t.TempDir()
	projectA := "/Users/alice/work/myapp"
	projectB := "/Users/alice/personal/myapp"

	dirA, err := resolveWithBase(base, projectA)
	if err != nil {
		t.Fatalf("resolve project A: %v", err)
	}

	dirB, err := resolveWithBase(base, projectB)
	if err != nil {
		t.Fatalf("resolve project B: %v", err)
	}

	if dirA == dirB {
		t.Errorf("collision: both projects resolved to %q", dirA)
	}

	// First should be "myapp", second should be "myapp-2".
	if filepath.Base(dirA) != "myapp" {
		t.Errorf("project A slug = %q, want %q", filepath.Base(dirA), "myapp")
	}
	if filepath.Base(dirB) != "myapp-2" {
		t.Errorf("project B slug = %q, want %q", filepath.Base(dirB), "myapp-2")
	}

	// Both should have correct meta.json.
	checkMeta(t, dirA, projectA)
	checkMeta(t, dirB, projectB)
}

func TestResolveWithBase_FindsExistingByMeta(t *testing.T) {
	base := t.TempDir()
	projectRoot := "/Users/carol/code/api"

	// Pre-create the directory with a different slug name (simulating
	// a collision scenario where this project got slug "api-2").
	projectsDir := filepath.Join(base, "projects")
	slugDir := filepath.Join(projectsDir, "api-2")
	if err := os.MkdirAll(slugDir, 0755); err != nil {
		t.Fatal(err)
	}
	meta := projectMeta{Path: projectRoot}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(slugDir, "meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Resolve should find the existing directory by scanning meta.json files.
	dir, err := resolveWithBase(base, projectRoot)
	if err != nil {
		t.Fatalf("resolveWithBase: %v", err)
	}

	if dir != slugDir {
		t.Errorf("resolved to %q, want %q (existing dir with matching meta)", dir, slugDir)
	}
}

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myapp", "myapp"},
		{"my-app", "my-app"},
		{"my_app", "my_app"},
		{"My.App", "My.App"},
		{"", "_"},
		{".", "_"},
		{"..", "_"},
		{"foo/bar", "foobar"},
		{"foo\\bar", "foobar"},
		{"a b c", "a b c"},
		{"...hidden", "...hidden"},
		{string([]byte{0x00, 0x01}), "_"},
	}

	for _, tc := range tests {
		got := sanitizeSlug(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeSlug(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestResolveWithBase_MultipleCollisions(t *testing.T) {
	base := t.TempDir()

	// Create 3 projects all with basename "app".
	projects := []string{
		"/a/app",
		"/b/app",
		"/c/app",
	}
	expectedSlugs := []string{"app", "app-2", "app-3"}

	dirs := make([]string, len(projects))
	for i, p := range projects {
		d, err := resolveWithBase(base, p)
		if err != nil {
			t.Fatalf("resolve %q: %v", p, err)
		}
		dirs[i] = d
	}

	for i, d := range dirs {
		if filepath.Base(d) != expectedSlugs[i] {
			t.Errorf("project %q: slug = %q, want %q", projects[i], filepath.Base(d), expectedSlugs[i])
		}
	}

	// All dirs should be unique.
	seen := make(map[string]bool)
	for _, d := range dirs {
		if seen[d] {
			t.Errorf("duplicate directory: %s", d)
		}
		seen[d] = true
	}
}

// checkMeta verifies the meta.json in dir matches the expected path.
func checkMeta(t *testing.T, dir, expectedPath string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		t.Fatalf("reading meta.json in %s: %v", dir, err)
	}
	var meta projectMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing meta.json in %s: %v", dir, err)
	}
	if meta.Path != expectedPath {
		t.Errorf("meta.Path in %s = %q, want %q", dir, meta.Path, expectedPath)
	}
}
