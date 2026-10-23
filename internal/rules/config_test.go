package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditConfigKeepsTheFile(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".config", "swelo"), 0o755)
	os.WriteFile(ConfigPath(home), []byte("# my settings\nroots: [~/code] # where projects live\nrecent_days: 3\n"), 0o644)
	if err := SetConfig(home, "lang", "tr"); err != nil {
		t.Fatal(err)
	}
	if err := AddToConfig(home, "roots", "~/work"); err != nil {
		t.Fatal(err)
	}
	if err := AddToConfig(home, "roots", "~/work"); err != nil { // no duplicate
		t.Fatal(err)
	}
	if err := AddToConfig(home, "disable", "docker"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFromConfig(home, "roots", "~/code"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFromConfig(home, "roots", "~/nope"); err != ErrNotInConfig {
		t.Fatalf("remove missing: %v", err)
	}
	b, _ := os.ReadFile(ConfigPath(home))
	for _, want := range []string{"# my settings", "# where projects live", "recent_days: 3", "lang: tr", "roots: [~/work]", "- docker"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %q in:\n%s", want, b)
		}
	}
	if strings.Count(string(b), "~/work") != 1 || strings.Contains(string(b), "~/code") {
		t.Fatalf("list edits wrong:\n%s", b)
	}
	cfg, err := ReadConfig(home)
	if err != nil || cfg.Lang != "tr" || len(cfg.Roots) != 1 || *cfg.RecentDays != 3 {
		t.Fatalf("%+v %v", cfg, err)
	}
	// a missing file is created
	home2 := t.TempDir()
	if err := SetConfig(home2, "top", "20"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := ReadConfig(home2); cfg.Top != 20 {
		t.Fatalf("%+v", cfg)
	}
}
