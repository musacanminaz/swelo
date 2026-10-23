package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/musacanminaz/swelo/internal/analyze"
	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/uninstall"
)

func TestCleanJSON(t *testing.T) {
	i18n.Set("en")
	home, _ := filepath.EvalSymlinks(t.TempDir())
	proj := filepath.Join(home, "code", "app")
	os.MkdirAll(filepath.Join(proj, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(proj, "package.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(proj, "node_modules", "x", "f"), bytes.Repeat([]byte("a"), 10000), 0o644)
	old := time.Now().AddDate(0, 0, -30)
	for _, p := range []string{filepath.Join(proj, "package.json"), filepath.Join(proj, "node_modules", "x", "f"), filepath.Join(proj, "node_modules", "x"), filepath.Join(proj, "node_modules"), proj} {
		os.Chtimes(p, old, old)
	}
	var out bytes.Buffer
	o := cleanOpts{only: []string{"node"}, recentDays: -1, json: &out}
	if err := runClean(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{}, home, o, false); err != nil {
		t.Fatal(err)
	}
	var doc cleanJSON
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%v in:\n%s", err, out.String())
	}
	if !doc.DryRun || len(doc.Rules) != 1 || doc.Rules[0].ID != "node" || len(doc.Rules[0].Items) != 1 ||
		doc.Rules[0].Items[0].Path != filepath.Join(proj, "node_modules") || doc.SafeBytes == 0 || doc.SafeBytes != doc.Rules[0].Bytes {
		t.Fatalf("doc:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(proj, "node_modules")); err != nil {
		t.Fatal("dry run deleted")
	}
	// prompts are impossible with --json
	o.apply = true
	if err := runClean(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{}, home, o, false); err == nil {
		t.Fatal("--apply without --yes accepted")
	}
	o.pick, o.apply = true, false
	if err := runClean(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{}, home, o, false); err == nil {
		t.Fatal("--pick accepted")
	}
}

func TestStatusJSON(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	d := statusData(home, dir, time.Now())
	if d.Disk.TotalBytes == 0 || d.Cleanups == nil || d.Trend != nil {
		t.Fatalf("%+v", d)
	}
	var out bytes.Buffer
	writeJSON(&out, d)
	if !strings.Contains(out.String(), `"cleanups": []`) {
		t.Fatalf("empty cleanups must be a list:\n%s", out.String())
	}
}

func TestAnalyzeAndUninstallDocs(t *testing.T) {
	res := analyze.Result{Sizes: map[string]int64{"projects": 50, "other": 100}, BigFiles: []analyze.File{{Path: "/h/big", Size: 2e9}}, Skipped: []string{"/h/Library/Mail"}}
	proj := t.TempDir()
	results := []clean.Result{{Rule: rules.Rule{ID: "node", Kind: "artifact", Names: []string{"node_modules"}},
		Items: []clean.Item{{Path: proj + "/node_modules", Project: proj, Size: 2e7}}}}
	doc := analyzeDoc(res, results, map[string]int64{"projects": 20}, nil, "/h", false)
	if doc.HomeBytes != 150 || doc.Categories[0].ID != "other" || doc.Categories[1].FreeableBytes != 20 ||
		len(doc.BigFiles) != 1 || len(doc.Projects) != 1 || doc.Projects[0].Path != proj || len(doc.SkippedProtected) != 1 {
		t.Fatalf("%+v", doc)
	}
	u := packagesJSON([]uninstall.Package{{ID: "brew:x", Provider: "brew", Name: "x", Size: 5, Command: []string{"brew", "uninstall", "x"}}})
	var out bytes.Buffer
	writeJSON(&out, u)
	if !strings.Contains(out.String(), `"command": [`) || strings.Contains(out.String(), `"installed"`) {
		t.Fatalf("%s", out.String())
	}
}
