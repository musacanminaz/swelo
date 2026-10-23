package cli

import (
	"bytes"
	"strings"
	"testing"
)

// complete runs cobra's hidden __complete command and returns its lines.
func complete(t *testing.T, args ...string) []string {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // the pre-run writes a snapshot under home
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs(append([]string{"__complete"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(out.String()), "\n")
}

func TestCompletion(t *testing.T) {
	has := func(lines []string, want string) bool {
		for _, l := range lines {
			if l == want {
				return true
			}
		}
		return false
	}
	if lines := complete(t, "clean", ""); !has(lines, "node") || !has(lines, "docker") || !has(lines, "build") {
		t.Fatalf("clean args: %v", lines)
	}
	if lines := complete(t, "uninstall", "--only", ""); !has(lines, "brew") || !has(lines, "docker-image") {
		t.Fatalf("uninstall --only: %v", lines)
	}
	if lines := complete(t, "config", "set", "lang", ""); !has(lines, "tr") || has(lines, "roots") {
		t.Fatalf("config set lang: %v", lines)
	}
	if lines := complete(t, "config", "add", ""); !has(lines, "roots") || has(lines, "lang") {
		t.Fatalf("config add: %v", lines)
	}
	if lines := complete(t, "--lang", ""); !has(lines, "en") {
		t.Fatalf("--lang: %v", lines)
	}
}
