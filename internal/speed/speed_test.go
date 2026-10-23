package speed

import (
	"testing"
	"time"
)

const topOut = `Processes: 3 total
PID    PPID   MEM    %CPU
4311   1      8030M  0.0
2012   1886   875M   0.0
PID    PPID   MEM    %CPU
4311   1      8030M  12.5
2012   1886   875M+  0.0
485    1      1193M- 13.8
`

const psOut = `  4311   501 /Applications/Android Studio.app/Contents/jbr/Contents/Home/bin/java -cp x org.gradle.launcher.daemon.bootstrap.GradleDaemon 8.9
  2012   501 node /Users/me/.npm/_npx/9833c18b2d85bc59/node_modules/.bin/playwright-mcp
   485     0 /System/Library/PrivateFrameworks/SkyLight.framework/Resources/WindowServer
`

func TestSnapshotParsing(t *testing.T) {
	run := func(argv ...string) (string, error) {
		switch argv[0] {
		case "sysctl":
			return "17179869184\n8\ntotal = 16384.00M  used = 15700.88M  free = 683.12M  (encrypted)\n{ 21.49 14.85 13.69 }\n{ sec = 1787584797, usec = 876958 } Mon Aug 24 18:19:57 2026\n", nil
		case "top":
			return topOut, nil
		case "ps":
			return psOut, nil
		}
		t.Fatalf("unexpected %v", argv)
		return "", nil
	}
	s := Snapshot(run)
	if s.RAM != 17179869184 || s.Cores != 8 || s.SwapTotal != 16384<<20 || s.SwapUsed != parseSize("15700.88M") || s.Load[0] != 21.49 {
		t.Fatalf("%+v", s)
	}
	if s.Uptime < 24*time.Hour {
		t.Fatalf("uptime %v", s.Uptime)
	}
	// the second top sample wins (cpu 12.5), memory from top, command from ps
	if len(s.Procs) != 3 || s.Procs[0].PID != 4311 || s.Procs[0].Mem != 8030<<20 || s.Procs[0].CPU != 12.5 || s.Procs[0].PPID != 1 {
		t.Fatalf("procs %+v", s.Procs)
	}
	if s.Procs[0].App != "Android Studio · Gradle daemon" || s.Procs[2].App != "node · playwright-mcp" || s.Procs[1].App != "WindowServer" {
		t.Fatalf("apps %q %q %q", s.Procs[0].App, s.Procs[1].App, s.Procs[2].App)
	}
	if s.Apps[0].Name != "Android Studio · Gradle daemon" || s.Apps[0].Procs != 1 || s.Apps[1].Name != "WindowServer" {
		t.Fatalf("grouped %+v", s.Apps)
	}
}

func TestAppName(t *testing.T) {
	for args, want := range map[string]string{
		"/Applications/Google Chrome.app/Contents/Frameworks/Google Chrome Framework.framework/Helpers/Google Chrome Helper (Renderer).app/Contents/MacOS/Google Chrome Helper (Renderer) --type=renderer": "Google Chrome",
		"/System/Library/Frameworks/Virtualization.framework/Versions/A/XPCServices/com.apple.Virtualization.VirtualMachine.xpc/Contents/MacOS/com.apple.Virtualization.VirtualMachine":                    "Virtual machine (Docker, Parallels…)",
		"/Users/me/.nvm/versions/node/v22.14.0/bin/node /Users/me/app/node_modules/.bin/vite --port 3000":                                                                                                  "node · vite",
		"python3 manage.py runserver": "python3 · manage.py",
		"/usr/local/bin/claude":       "Claude Code",
		"/bin/zsh -l":                 "zsh",
	} {
		if got := AppName(args); got != want {
			t.Errorf("AppName(%q) = %q, want %q", args, got, want)
		}
	}
	if parseSize("1.5G") != 3<<29 || parseSize("683.12M") != parseSize("683.12M") || parseSize("0B") != 0 {
		t.Fatal("parseSize")
	}
}
