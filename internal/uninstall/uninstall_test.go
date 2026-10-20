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
	mkfile(t, filepath.Join(home, "android", "cmdline-tools", "latest", "bin", "sdkmanager"), "")
	mkfile(t, filepath.Join(home, "android", "build-tools", "34.0.0", "x"), "x")
	mkfile(t, filepath.Join(home, "code", "app", ".tool-versions"), "nodejs 22.14.0\npython 3.12.1\n")

	out := map[string]string{
		"rustup default":                     "stable-aarch64-apple-darwin (default)",
		"xcrun simctl runtime list -j":       `{"A":{"identifier":"A","version":"26.5","platformIdentifier":"com.apple.platform.iphonesimulator","sizeBytes":8494282293,"lastUsedAt":"2026-10-19T19:56:57Z","deletable":true}}`,
		"docker info":                        "ok",
		"docker ps -a --format {{.ImageID}}": "sha256:aaaaaaaaaaaa1111\n",
		"docker images --format {{json .}}":  `{"Repository":"postgres","Tag":"17","ID":"aaaaaaaaaaaa","Size":"450MB","Containers":"2"}` + "\n" + `{"Repository":"old","Tag":"1","ID":"bbbbbbbbbbbb","Size":"1.2GB","Containers":"0"}`,
		"ollama list":                        "NAME        ID      SIZE    MODIFIED\nllama3:8b   abc     4.7 GB  3 weeks ago\n",
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
		Look:       func(n string) bool { return n != "brew" && n != "npm" && n != "cursor" },
		JDKDir:     filepath.Join(home, "jdk"),
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
	if a := ps["android:build-tools;34.0.0"]; a.Command[1] != "--uninstall" || a.Command[2] != "build-tools;34.0.0" {
		t.Errorf("android = %+v", a)
	}
	for _, p := range ps {
		if len(p.Command) == 0 && p.Note == "" {
			t.Errorf("%s has no uninstall command and no explanation", p.ID)
		}
	}
}
