package clean

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/musacanminaz/swelo/internal/rules"
)

var month = 30 * 24 * time.Hour

func file(t *testing.T, p string, age time.Duration) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-age)
	os.Chtimes(p, ts, ts)
}

func dir(t *testing.T, p string, age time.Duration) {
	t.Helper()
	file(t, filepath.Join(p, "f"), age)
	ts := time.Now().Add(-age)
	os.Chtimes(p, ts, ts)
}

func find(rs []Result, id string) Result {
	for _, r := range rs {
		if r.Rule.ID == id {
			return r
		}
	}
	return Result{}
}

func paths(items []Item) map[string]string {
	m := map[string]string{}
	for _, it := range items {
		m[filepath.Base(filepath.Dir(it.Path))+"/"+filepath.Base(it.Path)] = it.Skip
	}
	return m
}

func TestScanAndApply(t *testing.T) {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	d := filepath.Join(home, "Desktop")

	file(t, filepath.Join(d, "old", "package.json"), month)
	dir(t, filepath.Join(d, "old", "node_modules"), month)

	file(t, filepath.Join(d, "fresh", "package.json"), month)
	dir(t, filepath.Join(d, "fresh", "node_modules"), month)
	file(t, filepath.Join(d, "fresh", "index.ts"), time.Hour)

	// a fresh build/ next to .dart_tool must not make the project "recent"
	file(t, filepath.Join(d, "fl", "pubspec.yaml"), month)
	dir(t, filepath.Join(d, "fl", ".dart_tool"), month)
	dir(t, filepath.Join(d, "fl", "build"), time.Hour)

	dir(t, filepath.Join(d, "notjs", "dist"), month)
	file(t, filepath.Join(d, "skipme", "package.json"), month)
	dir(t, filepath.Join(d, "skipme", "node_modules"), month)

	pw := filepath.Join(home, "Library", "Caches", "ms-playwright")
	for _, n := range []string{"chromium-1194", "chromium-1208", "ffmpeg-1011"} {
		dir(t, filepath.Join(pw, n), month)
	}
	mcp := filepath.Join(home, "Library", "Caches", "ms-playwright-mcp")
	dir(t, filepath.Join(mcp, "open"), month)
	os.Symlink(fmt.Sprintf("host-%d", os.Getpid()), filepath.Join(mcp, "open", "SingletonLock"))
	dir(t, filepath.Join(mcp, "closed"), month)
	dir(t, filepath.Join(mcp, "stale"), month)
	os.Symlink("host-999999", filepath.Join(mcp, "stale", "SingletonLock")) // dead pid

	file(t, filepath.Join(d, "served", "package.json"), month)
	dir(t, filepath.Join(d, "served", "node_modules"), month)
	dir(t, filepath.Join(home, "Downloads"), month)

	all, cfg, err := rules.Load(home)
	if err != nil {
		t.Fatal(err)
	}
	rs := sandboxed(t, all, home)
	cfg.Roots = []string{"~/Desktop"}
	cfg.Exclude = []string{"*/skipme/*"}
	e := New(home, rs, cfg)
	e.procs = "" // ignore processes running on the test machine
	e.cwds = []string{filepath.Join(d, "served", "src")}
	res := e.Scan()

	node := find(res, "node")
	if got := paths(node.Items); len(got) != 1 || got["old/node_modules"] != "" {
		t.Errorf("node items = %v", got)
	}
	skips := paths(node.Skips)
	if skips["fresh/node_modules"] == "" || skips["skipme/node_modules"] != "excluded in config" ||
		skips["served/node_modules"] != "a process is running in this project" {
		t.Errorf("node skips = %v", skips)
	}
	if got := paths(find(res, "flutter").Items); len(got) != 2 {
		t.Errorf("flutter items = %v (sibling build/ should not block .dart_tool)", got)
	}
	if got := paths(find(res, "playwright-browsers").Items); len(got) != 1 || got["ms-playwright/chromium-1194"] != "" {
		t.Errorf("playwright items = %v", got)
	}
	if got := paths(find(res, "playwright-mcp").Items); len(got) != 2 || got["ms-playwright-mcp/closed"] != "" || got["ms-playwright-mcp/stale"] != "" {
		t.Errorf("mcp items = %v", got)
	}
	if node.Size < 5000 {
		t.Errorf("node size = %d", node.Size)
	}

	var deleted []string
	e.Apply(res, false, func(r Result, p string, _ int64, err error) {
		if r.Rule.Kind != "command" && err == nil {
			deleted = append(deleted, p)
		}
	})
	for _, keep := range []string{"fresh/node_modules", "served/node_modules", "notjs/dist", "skipme/node_modules", "fl/pubspec.yaml"} {
		if _, err := os.Stat(filepath.Join(d, keep)); err != nil {
			t.Errorf("%s was deleted", keep)
		}
	}
	for _, keep := range []string{filepath.Join(pw, "chromium-1208"), filepath.Join(mcp, "open"), filepath.Join(home, "Downloads")} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s was deleted", keep)
		}
	}
	if _, err := os.Stat(filepath.Join(d, "old", "node_modules")); !os.IsNotExist(err) {
		t.Error("old/node_modules not deleted")
	}
	if len(deleted) != 6 {
		t.Errorf("deleted %d items: %v", len(deleted), deleted)
	}
}

func TestLatest(t *testing.T) {
	k := Latest([]string{"chromium-1194", "chromium-1248", "chromium-1223", "ffmpeg-1011", "29.0.2", "36.0.0", "34.0.0"}, 2)
	for _, n := range []string{"chromium-1248", "chromium-1223", "ffmpeg-1011", "36.0.0", "34.0.0"} {
		if !k[n] {
			t.Errorf("%s should be kept", n)
		}
	}
	if k["chromium-1194"] || k["29.0.2"] {
		t.Errorf("old versions kept: %v", k)
	}
}

// sandboxed keeps only rules that cannot reach outside the fake home: no
// command rules (they act on the real machine) and no absolute paths.
func sandboxed(t *testing.T, rs []rules.Rule, home string) []rules.Rule {
	t.Helper()
	var out []rules.Rule
	for _, r := range rs {
		if r.Kind == "command" {
			continue
		}
		ok := true
		for _, p := range r.Paths {
			if !strings.HasPrefix(rules.Expand(p, home), home+"/") {
				ok = false
			}
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}
