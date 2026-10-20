package uninstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkfile(t *testing.T, p, body string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeEnv(t *testing.T) *Env {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	mkfile(t, filepath.Join(home, ".nvm", "versions", "node", "v18.20.0", "bin", "node"), "x")
	mkfile(t, filepath.Join(home, ".nvm", "versions", "node", "v22.14.0", "bin", "node"), "x")
	mkfile(t, filepath.Join(home, ".nvm", "alias", "default"), "22\n")
	mkfile(t, filepath.Join(home, ".rustup", "toolchains", "nightly-aarch64-apple-darwin", "x"), "x")
	mkfile(t, filepath.Join(home, ".vscode", "extensions", "dbaeumer.vscode-eslint-3.0.34", "package.json"), "{}")
	mkfile(t, filepath.Join(home, ".vscode", "extensions", "anthropic.claude-code-2.1.301-darwin-arm64", "package.json"), "{}")
	mkfile(t, filepath.Join(home, "code", "legacy", ".nvmrc"), "v18\n")
	mkfile(t, filepath.Join(home, "jdk", "zulu-17.jdk", "x"), "x")
	mkfile(t, filepath.Join(home, "jdk", "temurin-21.jdk", "x"), "x")
	apps := filepath.Join(home, "Applications")
	for _, a := range []string{"Foo", "Bar", "Busy", "Pages"} {
		mkfile(t, filepath.Join(apps, a+".app", "Contents", "Info.plist"), "")
		mkfile(t, filepath.Join(apps, a+".app", "Contents", "MacOS", a), "bin")
	}
	lib := filepath.Join(home, "Library")
	mkfile(t, filepath.Join(lib, "Application Support", "com.foo.app", "db"), "x")
	mkfile(t, filepath.Join(lib, "Caches", "com.foo.app", "c"), "x")
	mkfile(t, filepath.Join(lib, "Preferences", "com.foo.app.plist"), "x")
	mkfile(t, filepath.Join(lib, "Containers", "com.foo.app", "x"), "x")
	mkfile(t, filepath.Join(lib, "Preferences", "com.other.plist"), "x")
	mkfile(t, filepath.Join(apps, "Docker.app", "Contents", "Info.plist"), "")
	mkfile(t, filepath.Join(lib, "Containers", "com.docker.docker", "Data", "Docker.raw"), "volumes")
	mkfile(t, filepath.Join(lib, "Caches", "com.docker.docker", "c"), "x")
	mkfile(t, filepath.Join(home, "android", "cmdline-tools", "latest", "bin", "sdkmanager"), "")
	mkfile(t, filepath.Join(home, "android", "build-tools", "34.0.0", "x"), "x")
	mkfile(t, filepath.Join(home, "code", "app", ".tool-versions"), "nodejs 22.14.0\npython 3.12.1\n")

	out := map[string]string{
		"rustup default":                    "stable-aarch64-apple-darwin (default)",
		"xcrun simctl runtime list -j":      `{"A":{"identifier":"A","version":"26.5","platformIdentifier":"com.apple.platform.iphonesimulator","sizeBytes":8494282293,"lastUsedAt":"2026-10-19T19:56:57Z","deletable":true}}`,
		"docker info":                       "ok",
		"docker images --format {{json .}}": `{"Repository":"postgres","Tag":"17","ID":"aaaaaaaaaaaa","Size":"450MB","Containers":"2"}` + "\n" + `{"Repository":"old","Tag":"1","ID":"bbbbbbbbbbbb","Size":"1.2GB","Containers":"0"}`,
		"brew --cellar":                     filepath.Join(home, "cellar"),
		"brew --caskroom":                   filepath.Join(home, "caskroom"),
		"brew list --cask":                  "temurin@21\nfirefox\n",
		"plutil -extract CFBundleIdentifier raw " + filepath.Join(home, "Applications", "Foo.app", "Contents", "Info.plist"):    "com.foo.app\n",
		"plutil -extract CFBundleIdentifier raw " + filepath.Join(home, "Applications", "Bar.app", "Contents", "Info.plist"):    "com.bar\n",
		"plutil -extract CFBundleIdentifier raw " + filepath.Join(home, "Applications", "Busy.app", "Contents", "Info.plist"):   "com.busy\n",
		"plutil -extract CFBundleIdentifier raw " + filepath.Join(home, "Applications", "Pages.app", "Contents", "Info.plist"):  "com.apple.iWork.Pages\n",
		"plutil -extract CFBundleIdentifier raw " + filepath.Join(home, "Applications", "Docker.app", "Contents", "Info.plist"): "com.docker.docker\n",
		"brew info --cask --json=v2 --installed": `{"casks":[{"token":"bar-app","artifacts":[{"app":["Bar.app"]}]}]}`,
		"ollama list":                            "NAME        ID      SIZE    MODIFIED\nllama3:8b   abc     4.7 GB  3 weeks ago\n",
	}
	return &Env{
		Home:  home,
		Roots: []string{filepath.Join(home, "code")},
		Run: func(argv ...string) (string, error) {
			if v, ok := out[strings.Join(argv, " ")]; ok {
				return v, nil
			}
			return "", os.ErrNotExist
		},
		Look:       func(n string) bool { return n != "npm" && n != "cursor" },
		JDKDir:     filepath.Join(home, "jdk"),
		AppDirs:    []string{filepath.Join(home, "Applications")},
		Running:    func(app string) bool { return strings.HasSuffix(app, "Busy.app") },
		AndroidSDK: filepath.Join(home, "android"),
	}
}

func byID(ps []Package) map[string]Package {
	m := map[string]Package{}
	for _, p := range ps {
		m[p.ID] = p
	}
	return m
}

func TestList(t *testing.T) {
	ps := byID(List(fakeEnv(t), nil))

	n18, n22 := ps["node:v18.20.0"], ps["node:v22.14.0"]
	if len(n18.UsedBy) != 1 || !strings.HasSuffix(n18.UsedBy[0], "legacy") {
		t.Errorf("node 18 used by = %v", n18.UsedBy)
	}
	if n22.Note != "nvm default" || len(n22.UsedBy) != 1 {
		t.Errorf("node 22 = %+v", n22)
	}
	if got := ps["rust:nightly-aarch64-apple-darwin"]; got.Note != "" || got.Command[3] != "nightly-aarch64-apple-darwin" {
		t.Errorf("rust = %+v", got)
	}
	sim := ps["simruntime:A"]
	if sim.Size != 8494282293 || sim.LastUsed.IsZero() || sim.Name != "iOS 26.5 simulator runtime" {
		t.Errorf("sim = %+v", sim)
	}
	if ps["docker:aaaaaaaaaaaa"].Note == "" || ps["docker:bbbbbbbbbbbb"].Note != "" || ps["docker:bbbbbbbbbbbb"].Size != 1.2e9 {
		t.Errorf("docker = %+v / %+v", ps["docker:aaaaaaaaaaaa"], ps["docker:bbbbbbbbbbbb"])
	}
	if got := ps["code:anthropic.claude-code-2.1.301-darwin-arm64"]; got.Command[2] != "anthropic.claude-code" {
		t.Errorf("vscode id = %v", got.Command)
	}
	if got := ps["ollama:llama3:8b"]; got.Size != 4.7e9 {
		t.Errorf("ollama = %+v", got)
	}
	if j := ps["jdk:zulu-17.jdk"]; !j.Sudo || j.Command[0] != "rm" {
		t.Errorf("jdk = %+v", j)
	}
	if j := ps["jdk:temurin-21.jdk"]; j.Sudo || strings.Join(j.Command, " ") != "brew uninstall --cask temurin@21" {
		t.Errorf("cask jdk = %+v", j)
	}
	if _, dup := ps["cask:temurin@21"]; dup {
		t.Error("cask-installed JDK listed twice")
	}
	if _, ok := ps["cask:firefox"]; !ok {
		t.Error("other casks must still be listed")
	}
	if a := ps["android:build-tools;34.0.0"]; a.Command[1] != "--uninstall" || a.Command[2] != "build-tools;34.0.0" {
		t.Errorf("android = %+v", a)
	}
	foo := ps["app:Foo.app"]
	cmd := strings.Join(foo.Command, " ")
	if foo.Command[0] != "/usr/bin/trash" || !strings.Contains(cmd, "Application Support/com.foo.app") ||
		!strings.Contains(cmd, "com.foo.app.plist") || strings.Contains(cmd, "com.other") {
		t.Errorf("foo = %v", foo.Command)
	}
	if strings.Contains(cmd, "Containers") {
		t.Error("app Containers must not be touched without Full Disk Access")
	}
	if b := ps["app:Bar.app"]; strings.Join(b.Command, " ") != "brew uninstall --cask bar-app" {
		t.Errorf("bar = %+v", b)
	}
	if _, dup := ps["cask:bar-app"]; dup {
		t.Error("cask app listed twice")
	}
	e2 := fakeEnv(t)
	e2.FDA = true
	for _, p := range List(e2, []string{"app"}) {
		if p.ID == "app:Docker.app" {
			c := strings.Join(p.Command, " ")
			if strings.Contains(c, "Containers/com.docker.docker") || !strings.Contains(c, "Caches/com.docker.docker") {
				t.Errorf("docker leftovers = %v (its container holds every volume)", p.Command)
			}
		}
	}
	if b := ps["app:Busy.app"]; b.Command != nil {
		t.Errorf("running app must not be removable: %+v", b)
	}
	if _, ok := ps["app:Pages.app"]; ok {
		t.Error("Apple apps must not be listed")
	}
	for _, p := range ps {
		if len(p.Command) == 0 && p.Note == "" {
			t.Errorf("%s has no uninstall command and no explanation", p.ID)
		}
	}
}

func TestRemove(t *testing.T) {
	log := filepath.Join(t.TempDir(), "history.jsonl")
	pkgs := []Package{
		{ID: "brew:a", Command: []string{"brew", "uninstall", "a"}},
		{ID: "jdk:x", Command: []string{"rm", "-rf", "/Library/Java/JavaVirtualMachines/x.jdk"}, Sudo: true},
		{ID: "sim:n", Note: "not deletable"},
	}
	var ran [][]string
	var sudoed []string
	out := Remove(pkgs,
		func(a []string) error { ran = append(ran, a); return nil },
		func(p Package) error { sudoed = append(sudoed, p.ID); return nil }, log)
	if len(ran) != 1 || ran[0][2] != "a" {
		t.Errorf("ran = %v", ran)
	}
	if len(sudoed) != 1 || sudoed[0] != "jdk:x" {
		t.Errorf("sudo = %v (sudo packages must never go through the plain runner)", sudoed)
	}
	if out[2].Err == nil {
		t.Error("package without a command must fail")
	}
	b, _ := os.ReadFile(log)
	if strings.Count(string(b), "\n") != 3 {
		t.Errorf("log = %s", b)
	}
}
