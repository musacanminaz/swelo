package speed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStaleProcesses(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".Trash", "old-app"), 0o755)
	live := filepath.Join(home, "live")
	os.Mkdir(live, 0o755)
	me := os.Getuid()
	procs := []Proc{
		{PID: 10, PPID: 9, UID: me, Age: time.Hour, Args: "node server.js"}, // cwd in Trash
		{PID: 11, PPID: 9, UID: me, Age: time.Hour, Args: "/bin/zsh -l"},    // a shell in the Trash is not offered
		{PID: 12, PPID: 9, UID: me, Age: time.Hour, Args: "python3 app.py"}, // cwd deleted
		{PID: 13, PPID: 1, UID: me, Age: 3 * time.Hour, CPU: 0.2, Args: "/x/bin/java -cp y org.gradle.launcher.daemon.bootstrap.GradleDaemon"},
		{PID: 14, PPID: 1, UID: me, Age: 3 * time.Hour, CPU: 40, Args: "/x/bin/java -cp y org.gradle.launcher.daemon.bootstrap.GradleDaemon"}, // busy
		{PID: 15, PPID: 20, UID: me, Age: 30 * time.Hour, Args: "/Users/me/Library/Caches/ms-playwright/chromium-1/chrome-mac/Chromium.app/Contents/MacOS/Chromium --user-data-dir=/x"},
		{PID: 16, PPID: 20, UID: me, Age: time.Hour, Args: "/Users/me/Library/Caches/ms-playwright/chromium-1/chrome --headless=new"}, // young
		{PID: 22, PPID: 1, UID: me, Age: 3 * time.Hour, CPU: 0.1, Args: "/Applications/Android Studio.app/Contents/jbr/Contents/Home/bin/java -cp y org.gradle.launcher.daemon.bootstrap.GradleDaemon"},
		{PID: 23, PPID: 1, UID: me, Age: 40 * time.Hour, Args: "/Applications/Visual Studio Code.app/Contents/Frameworks/Code Helper.app/Contents/MacOS/Code Helper --type=utility --headless"}, // not a browser
		{PID: 24, PPID: 1, UID: me, Age: 40 * time.Hour, Args: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome --user-data-dir=/Users/me/Library/Caches/ms-playwright-mcp/mcp-chrome-1"},
		{PID: 25, PPID: 1, UID: me, Age: 40 * time.Hour, Args: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}, // the user's own Chrome
		{PID: 26, PPID: 24, UID: me, Age: 40 * time.Hour, Mem: 100, Args: "/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Helpers/Google Chrome Helper (Renderer).app/Contents/MacOS/Google Chrome Helper (Renderer) --type=renderer --user-data-dir=/Users/me/Library/Caches/ms-playwright-mcp/mcp-chrome-1"}, // folded into 24
		{PID: 17, PPID: 1, UID: me, Age: 50 * time.Hour, Args: "node /x/.bin/playwright-mcp"},  // orphan
		{PID: 18, PPID: 30, UID: me, Age: 50 * time.Hour, Args: "node /x/.bin/playwright-mcp"}, // parent alive
		{PID: 19, PPID: 9, UID: me + 1, Age: time.Hour, Args: "node server.js"},                // not mine
		{PID: 21, PPID: 9, UID: me, Age: time.Hour, Args: "node dev"},                          // cwd live
	}
	lsof := "p10\nfcwd\nn" + filepath.Join(home, ".Trash", "old-app") + "\np11\nfcwd\nn" + filepath.Join(home, ".Trash") +
		"\np12\nfcwd\nn" + filepath.Join(home, "gone") + "\np19\nfcwd\nn" + filepath.Join(home, ".Trash") + "\np21\nfcwd\nn" + live + "\n"
	run := func(argv ...string) (string, error) { return lsof, nil }
	got := map[int]string{}
	for _, s := range StaleProcesses(run, procs, home) {
		got[s.PID] = s.Reason
		if s.PID == 24 && (s.Mem != 100 || s.Children != 1) {
			t.Errorf("helper not folded into the browser: %+v", s)
		}
	}
	want := map[int]string{10: ReasonTrash, 12: ReasonGone, 13: ReasonDaemon, 15: ReasonBrowser, 17: ReasonOrphan, 22: ReasonDaemon, 24: ReasonBrowser}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for pid, r := range want {
		if got[pid] != r {
			t.Errorf("pid %d: %q, want %q", pid, got[pid], r)
		}
	}
	if parseEtime("04-06:28:45") != 4*24*time.Hour+6*time.Hour+28*time.Minute+45*time.Second || parseEtime("05:12") != 5*time.Minute+12*time.Second {
		t.Fatal("parseEtime")
	}
}
