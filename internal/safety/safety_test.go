package safety

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mk(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, p string, age time.Duration) {
	t.Helper()
	mk(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-age)
	os.Chtimes(p, ts, ts)
}

func old(t *testing.T, p string) {
	t.Helper()
	ts := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(p, ts, ts)
}

func setup(t *testing.T) (*Guard, string) {
	t.Helper()
	home, _ := filepath.EvalSymlinks(t.TempDir())
	projects := filepath.Join(home, "Projects")
	mk(t, projects)
	g := &Guard{
		Home:       home,
		Roots:      []string{projects, filepath.Join(home, "Library", "Caches")},
		Protected:  []string{filepath.Join(home, "Projects", "keep")},
		RecentDays: 7,
		LogPath:    filepath.Join(home, ".local", "state", "swelo", "history.jsonl"),
	}
	return g, projects
}

// oldProject creates a project whose files are all 30 days old.
func oldProject(t *testing.T, dir string) {
	t.Helper()
	touch(t, filepath.Join(dir, "package.json"), 30*24*time.Hour)
	mk(t, filepath.Join(dir, "node_modules", "left-pad"))
	touch(t, filepath.Join(dir, "node_modules", "left-pad", "index.js"), 0)
	old(t, filepath.Join(dir, "node_modules"))
}

func TestAllowsOldArtifactAndLogs(t *testing.T) {
	g, projects := setup(t)
	app := filepath.Join(projects, "app")
	oldProject(t, app)
	nm := filepath.Join(app, "node_modules")
	if err := g.Remove(nm, app, 123, "build", false); err != nil {
		t.Fatalf("expected removal, got %v", err)
	}
	if _, err := os.Stat(nm); !os.IsNotExist(err) {
		t.Fatal("node_modules still exists")
	}
	log, _ := os.ReadFile(g.LogPath)
	if !strings.Contains(string(log), nm) {
		t.Fatalf("log missing path: %s", log)
	}
}

func TestDryRunDeletesNothing(t *testing.T) {
	g, projects := setup(t)
	app := filepath.Join(projects, "app")
	oldProject(t, app)
	nm := filepath.Join(app, "node_modules")
	if err := g.Remove(nm, app, 0, "build", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(nm); err != nil {
		t.Fatal("dry run deleted the directory")
	}
}

func TestRefusals(t *testing.T) {
	g, projects := setup(t)
	home := g.Home

	// recently changed project
	fresh := filepath.Join(projects, "fresh")
	oldProject(t, fresh)
	touch(t, filepath.Join(fresh, "src.ts"), time.Hour)

	// git-tracked dist
	tracked := filepath.Join(projects, "tracked")
	oldProject(t, tracked)
	touch(t, filepath.Join(tracked, "dist", "app.js"), 30*24*time.Hour)
	for _, args := range [][]string{{"init", "-q"}, {"add", "dist/app.js"}} {
		cmd := exec.Command("git", append([]string{"-C", tracked}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	ts := time.Now().Add(-30 * 24 * time.Hour)
	os.Chtimes(filepath.Join(tracked, ".git", "index"), ts, ts)

	// symlink escaping the roots
	outside := filepath.Join(home, "Documents", "thesis")
	mk(t, outside)
	link := filepath.Join(projects, "linked")
	os.Symlink(filepath.Join(home, "Documents"), link)

	touch(t, filepath.Join(projects, "app", ".env.local"), 30*24*time.Hour)
	touch(t, filepath.Join(projects, "app", "backup.sql"), 30*24*time.Hour)
	mk(t, filepath.Join(projects, "keep", "node_modules"))
	mk(t, filepath.Join(projects, "repo", ".git", "objects"))

	cases := map[string]struct{ path, project string }{
		"recent project": {filepath.Join(fresh, "node_modules"), fresh},
		"git tracked":    {filepath.Join(tracked, "dist"), tracked},
		"symlink escape": {filepath.Join(link, "thesis"), ""},
		"outside roots":  {outside, ""},
		"home":           {home, ""},
		"root itself":    {projects, ""},
		"relative":       {"Projects/app", ""},
		"env file":       {filepath.Join(projects, "app", ".env.local"), ""},
		"sql dump":       {filepath.Join(projects, "app", "backup.sql"), ""},
		"protected":      {filepath.Join(projects, "keep", "node_modules"), ""},
		"inside .git":    {filepath.Join(projects, "repo", ".git", "objects"), ""},
		"missing":        {filepath.Join(projects, "nope"), ""},
	}
	for name, c := range cases {
		err := g.Remove(c.path, c.project, 0, "test", false)
		if !IsRefusal(err) {
			t.Errorf("%s: expected refusal, got %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outside)); err != nil {
		t.Fatal("outside directory was touched")
	}
	if _, err := os.Stat(filepath.Join(tracked, "dist", "app.js")); err != nil {
		t.Fatal("tracked dist was deleted")
	}
}
