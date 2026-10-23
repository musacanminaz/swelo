package startup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanAndCommands(t *testing.T) {
	home := t.TempDir()
	agents := filepath.Join(home, "Library", "LaunchAgents")
	os.MkdirAll(agents, 0o755)
	app := filepath.Join(home, "Apps", "Live.app", "Contents", "MacOS")
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(app, "Live"), nil, 0o755)
	for _, n := range []string{"com.live.agent.plist", "com.gone.helper.plist", "com.google.keystone.agent.plist", "com.apple.foo.plist", "notes.txt"} {
		os.WriteFile(filepath.Join(agents, n), nil, 0o644)
	}
	plists := map[string]string{
		"com.live.agent.plist":            `{"Label":"com.live.agent","ProgramArguments":["` + filepath.Join(app, "Live") + `","--background"],"RunAtLoad":true,"KeepAlive":{"SuccessfulExit":false}}`,
		"com.gone.helper.plist":           `{"Label":"com.gone.helper","Program":"/Applications/Gone.app/Contents/MacOS/Gone","RunAtLoad":true}`,
		"com.google.keystone.agent.plist": `{}`,
	}
	run := func(argv ...string) (string, error) {
		switch argv[0] {
		case "plutil":
			return plists[filepath.Base(argv[len(argv)-1])], nil
		case "launchctl":
			if argv[1] == "list" {
				return "PID\tStatus\tLabel\n123\t0\tcom.live.agent\n-\t0\tcom.gone.helper\n", nil
			}
			return "\tdisabled services = {\n\t\t\"com.gone.helper\" => disabled\n\t\t\"com.live.agent\" => enabled\n\t}\n", nil
		}
		return "", nil
	}
	items := Scan(run, home, 501)
	byLabel := map[string]Item{}
	for _, it := range items {
		if it.Kind != UserAgent { // the real /Library folders of this Mac are read too; only check ours
			continue
		}
		byLabel[it.Label] = it
	}
	if len(byLabel) != 3 {
		t.Fatalf("user agents: %+v", byLabel)
	}
	live := byLabel["com.live.agent"]
	if live.App != "Live" || live.Orphan || !live.Running || live.PID != 123 || !live.RunAtLoad || !live.KeepAlive || live.Disabled {
		t.Fatalf("live: %+v", live)
	}
	gone := byLabel["com.gone.helper"]
	if gone.App != "Gone" || !gone.Orphan || gone.Running || !gone.Disabled {
		t.Fatalf("gone: %+v", gone)
	}
	ks := byLabel["com.google.keystone.agent"]
	if !ks.Empty || !ks.Orphan || !ks.Updater {
		t.Fatalf("keystone: %+v", ks)
	}
	off, undo, sudo := Commands(live, 501)
	if sudo || len(off) != 2 || off[0][2] != "gui/501/com.live.agent" || !strings.Contains(undo, "bootstrap gui/501 "+live.File) {
		t.Fatalf("agent commands: %v %q %v", off, undo, sudo)
	}
	off, undo, sudo = Commands(Item{Label: "com.d", Kind: Daemon, File: "/Library/LaunchDaemons/com.d.plist"}, 501)
	if !sudo || off[1][2] != "system/com.d" || !strings.HasPrefix(undo, "sudo launchctl enable system/com.d") {
		t.Fatalf("daemon commands: %v %q %v", off, undo, sudo)
	}
}
