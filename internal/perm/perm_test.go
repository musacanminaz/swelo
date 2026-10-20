package perm

import (
	"os"
	"os/exec"
	"strconv"
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

// A background job started from the same terminal is reported; the test's own
// ancestors are not.
func TestTerminalNeighbours(t *testing.T) {
	if out, _ := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(os.Getpid())).Output(); strings.TrimSpace(string(out)) == "??" {
		t.Skip("no terminal")
	}
	bg := exec.Command("sleep", "30")
	if err := bg.Start(); err != nil {
		t.Fatal(err)
	}
	defer bg.Process.Kill()
	found := false
	for _, p := range TerminalNeighbours() {
		found = found || strings.HasSuffix(p, " sleep")
		if strings.HasSuffix(p, ".test") || strings.HasSuffix(p, " perm.test") {
			t.Errorf("own process listed: %s", p)
		}
	}
	if !found {
		t.Errorf("sleep not listed: %v", TerminalNeighbours())
	}
}
