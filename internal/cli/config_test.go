package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
)

func TestConfigCommands(t *testing.T) {
	i18n.Set("en")
	home := t.TempDir()
	var out bytes.Buffer
	if err := setConfig(&out, home, "lang", "fr"); err == nil {
		t.Fatal("bad language accepted")
	}
	if err := setConfig(&out, home, "roots", "~/x"); err == nil {
		t.Fatal("list key accepted by set")
	}
	if err := setConfig(&out, home, "recent_days", "x"); err == nil {
		t.Fatal("non-number accepted")
	}
	if err := setConfig(&out, home, "lang", "tr"); err != nil {
		t.Fatal(err)
	}
	if err := editList(&out, home, "roots", "~/missing", true); err == nil {
		t.Fatal("missing folder accepted as root")
	}
	os.Mkdir(filepath.Join(home, "code"), 0o755)
	if err := editList(&out, home, "roots", "~/code", true); err != nil {
		t.Fatal(err)
	}
	if err := editList(&out, home, "disable", "nope", true); err == nil {
		t.Fatal("unknown rule accepted")
	}
	if err := editList(&out, home, "disable", "docker", true); err != nil {
		t.Fatal(err)
	}
	if err := editList(&out, home, "roots", "~/other", false); err == nil || !strings.Contains(err.Error(), "~/other") {
		t.Fatalf("remove missing: %v", err)
	}
	cfg, _ := rules.ReadConfig(home)
	if cfg.Lang != "tr" || len(cfg.Roots) != 1 || cfg.Roots[0] != "~/code" || len(cfg.Disable) != 1 {
		t.Fatalf("%+v", cfg)
	}
	out.Reset()
	showConfig(&out, home)
	if !strings.Contains(out.String(), "lang: tr") || !strings.Contains(out.String(), rules.ConfigPath(home)) {
		t.Fatalf("%s", out.String())
	}
}
