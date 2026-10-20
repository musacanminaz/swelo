package perm

import (
	"os/exec"
	"strings"
	"testing"
)

// While admin rights are granted, child processes run without the terminal
// that sudo's cached credentials belong to.
func TestIsolate(t *testing.T) {
	if c := Isolate(exec.Command("true")); c.SysProcAttr != nil && c.SysProcAttr.Setsid {
		t.Fatal("isolated without a sudo session")
	}
	SetSudoSession(true)
	defer SetSudoSession(false)
	out, err := Isolate(exec.Command("sh", "-c", "ps -o tty= -p $$")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if tty := strings.TrimSpace(string(out)); tty != "??" {
		t.Fatalf("child still has a controlling terminal: %q", tty)
	}
}
