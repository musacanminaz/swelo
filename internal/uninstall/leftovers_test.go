package uninstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLeftovers(t *testing.T) {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	apps := filepath.Join(home, "Applications")
	mkfile(t, filepath.Join(apps, "Live.app", "Contents", "Info.plist"), "")
	mkfile(t, filepath.Join(apps, "Utilities", "Tool.app", "Contents", "Info.plist"), "")
	lib := filepath.Join(home, "Library")
	old := time.Now().Add(-60 * 24 * time.Hour)
	age := func(p string) { os.Chtimes(p, old, old) }
	for _, p := range []string{
		"Application Support/com.gone.app/db", "Caches/com.gone.app/c", "Preferences/com.gone.app.plist", // gone: three folders, one package
		"Application Support/com.live.app.helper/x", // helper of an installed app
		"Caches/com.tool.cli/x",                     // installed in a subfolder
		"Application Support/com.elsewhere.app/x",   // Spotlight finds the app elsewhere
		"Caches/com.fresh.app/x",                    // written to recently
		"Application Support/com.apple.foo/x",       // Apple's
		"Application Support/Slack/x",               // a plain name, not judged
	} {
		mkfile(t, filepath.Join(lib, p), "x")
	}
	for _, p := range []string{"Application Support/com.gone.app", "Caches/com.gone.app", "Preferences/com.gone.app.plist",
		"Application Support/com.live.app.helper", "Caches/com.tool.cli", "Application Support/com.elsewhere.app", "Application Support/com.apple.foo"} {
		age(filepath.Join(lib, p))
	}
	e := &Env{Home: home, AppDirs: []string{apps}, Run: func(argv ...string) (string, error) {
		cmd := strings.Join(argv, " ")
		switch {
		case strings.Contains(cmd, "Live.app"):
			return "com.live.app\n", nil
		case strings.Contains(cmd, "Tool.app"):
			return "com.tool.cli\n", nil
		case argv[0] == "mdfind" && strings.Contains(cmd, "com.elsewhere.app"):
			return "/Users/me/Downloads/Elsewhere.app\n", nil
		case argv[0] == "mdfind":
			return "", nil
		}
		return "", os.ErrNotExist
	}}
	got := leftovers(e)
	if len(got) != 1 || got[0].ID != "leftover:com.gone.app" || len(got[0].Command) != 4 || got[0].Command[0] != "/usr/bin/trash" {
		t.Fatalf("%+v", got)
	}
	if got[0].Size == 0 || got[0].Installed.After(old.Add(time.Second)) {
		t.Fatalf("size %d installed %v", got[0].Size, got[0].Installed)
	}
}
