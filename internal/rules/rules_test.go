package rules

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestProjectRootsHomeGlob(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{"code", "Desktop", "Library", "Applications", ".vscode", "Documents", "OneDrive - Acme"} {
		os.Mkdir(filepath.Join(home, d), 0o755)
	}
	os.WriteFile(filepath.Join(home, "notes.txt"), nil, 0o644)
	os.Symlink(filepath.Join(home, "code"), filepath.Join(home, "link"))
	got := ProjectRoots(Config{Roots: []string{"~/*", "~/missing"}, Protected: []string{"~/Documents"}}, home)
	want := []string{filepath.Join(home, "Desktop"), filepath.Join(home, "code")}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
