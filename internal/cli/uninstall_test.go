package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/uninstall"
)

func TestReviewAndRemove(t *testing.T) {
	i18n.Set("en")
	pkgs := []uninstall.Package{
		{ID: "node:v18", Name: "Node v18", Size: 100, Command: []string{"bash", "-c", "nvm uninstall", "nvm", "v18"}, UsedBy: []string{"/p/legacy"}},
		{ID: "jdk:x", Name: "JDK x", Size: 50, Sudo: true, Command: []string{"rm", "-rf", "/Library/Java/JavaVirtualMachines/x.jdk"}},
	}
	state := t.TempDir()
	var ran [][]string
	run := func(a []string) error { ran = append(ran, a); return nil }

	// declined: nothing runs, but every command was shown first
	var out bytes.Buffer
	reviewAndRemove(bufio.NewReader(strings.NewReader("n\n")), &out, pkgs, uninstallOpts{}, run, state)
	if len(ran) != 0 {
		t.Fatalf("ran without confirmation: %v", ran)
	}
	for _, want := range []string{"$ bash -c 'nvm uninstall' nvm v18", "$ sudo rm -rf /Library/Java/JavaVirtualMachines/x.jdk", "pinned by /p/legacy"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("review missing %q:\n%s", want, out.String())
		}
	}

	// --yes runs the normal one; the sudo step still asks and the empty answer skips it
	out.Reset()
	reviewAndRemove(bufio.NewReader(strings.NewReader("\n")), &out, pkgs, uninstallOpts{yes: true}, run, state)
	if len(ran) != 1 || ran[0][0] != "bash" {
		t.Fatalf("ran = %v (sudo must not be auto-approved by --yes)", ran)
	}
	if !strings.Contains(out.String(), "Removed 1 of 2") {
		t.Errorf("summary:\n%s", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(state, "history.jsonl")); strings.Count(string(b), "\n") != 2 {
		t.Errorf("log = %s", b)
	}
}
