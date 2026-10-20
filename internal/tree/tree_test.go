package tree

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/musacanminaz/swelo/internal/i18n"
)

func write(t *testing.T, p string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, make([]byte, n), 0o644)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestBuildAndBrowse(t *testing.T) {
	i18n.Set("en")
	home, _ := filepath.EvalSymlinks(t.TempDir())
	write(t, filepath.Join(home, "big", "a.bin"), 3<<20)
	write(t, filepath.Join(home, "big", "tiny.txt"), 10)
	write(t, filepath.Join(home, "small", "b.bin"), 2<<20)
	write(t, filepath.Join(home, "Library", "Caches", "x.bin"), 1<<20)
	write(t, filepath.Join(home, "secret", "x.bin"), 1<<20)

	root := Build(home, []string{filepath.Join(home, "secret")})
	if root.Children[0].Name != "big" || root.Children[0].Small != 1 {
		t.Fatalf("children not sorted/folded: %+v", root.Children[0])
	}

	var trashed []string
	b := NewBrowser(root, home, []string{filepath.Join(home, "secret")})
	b.TrashFn = func(p string) error { trashed = append(trashed, p); return nil }
	before := root.Size

	b.Update(key("enter")) // into big/
	if b.cur.Name != "big" {
		t.Fatalf("enter did not open big/: %s", b.cur.Name)
	}
	b.Update(key("d"))
	b.Update(key("n")) // cancel
	if len(trashed) != 0 {
		t.Fatal("trashed without confirmation")
	}
	b.Update(key("d"))
	b.Update(key("y"))
	if len(trashed) != 1 || filepath.Base(trashed[0]) != "a.bin" {
		t.Fatalf("trashed = %v", trashed)
	}
	if root.Size >= before || b.Freed < 3<<20 {
		t.Fatalf("sizes not updated: before %d after %d freed %d", before, root.Size, b.Freed)
	}
	b.Update(key("left"))
	for _, c := range root.Children {
		if c.Name == "Library" {
			if CanTrash(filepath.Join(c.Path, "Caches"), home, home, nil) == "" {
				t.Fatal("~/Library/Caches must not be trashable from the browser")
			}
		}
	}
	if CanTrash(home, home, home, nil) == "" || CanTrash("/etc", home, home, nil) == "" ||
		CanTrash(filepath.Join(home, "secret", "x.bin"), home, home, []string{filepath.Join(home, "secret")}) == "" {
		t.Fatal("refusals missing")
	}
	if CanTrash(filepath.Join(home, "small"), home, home, nil) != "" {
		t.Fatal("normal folder should be trashable")
	}
}
