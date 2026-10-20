package ui

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
)

func in(s string) *bufio.Reader { return bufio.NewReader(strings.NewReader(s)) }

func TestConfirm(t *testing.T) {
	i18n.Set("en")
	cases := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true}, {"evet\n", false, true}, {"E\n", false, true},
		{"n\n", true, false}, {"\n", false, false}, {"\n", true, true},
		{"", false, false}, {"", true, true}, {"maybe\n", true, false},
	}
	for _, c := range cases {
		if got := Confirm(in(c.input), &bytes.Buffer{}, "ok?", c.def); got != c.want {
			t.Errorf("Confirm(%q, def=%v) = %v", c.input, c.def, got)
		}
	}
}

func TestSudoStep(t *testing.T) {
	i18n.Set("en")
	step := SudoStep{Action: "Remove JDK", Reason: "root-owned", Command: []string{"rm", "-rf", "/Library/Java/JavaVirtualMachines/my jdk"}}

	var ran [][]string
	run := func(a []string) error { ran = append(ran, a); return nil }

	var out bytes.Buffer
	if err := RunSudo(in("\n"), &out, step, SudoOptions{Run: run}); err != ErrSkipped || len(ran) != 0 {
		t.Fatalf("empty answer must skip, got err=%v ran=%v", err, ran)
	}
	if !strings.Contains(out.String(), "sudo rm -rf '/Library/Java/JavaVirtualMachines/my jdk'") {
		t.Fatalf("exact command not shown:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "cannot be undone") {
		t.Fatalf("missing undo line:\n%s", out.String())
	}
	if err := RunSudo(in("y\n"), &bytes.Buffer{}, step, SudoOptions{NoSudo: true, Run: run}); err != ErrSkipped || len(ran) != 0 {
		t.Fatal("--no-sudo must skip even when the user says yes")
	}
	if err := RunSudo(in("y\n"), &bytes.Buffer{}, step, SudoOptions{Run: run}); err != nil || len(ran) != 1 {
		t.Fatalf("yes must run once, err=%v ran=%v", err, ran)
	}
}

func TestPad(t *testing.T) {
	if got := Pad("Çöp", 5); got != "Çöp  " {
		t.Errorf("Pad = %q", got)
	}
	if got := Pad("\x1b[1mab\x1b[0m", 4); got != "\x1b[1mab\x1b[0m  " {
		t.Errorf("Pad with color = %q", got)
	}
}

func TestSafe(t *testing.T) {
	for in, want := range map[string]string{
		"node_modules":              "node_modules",
		"Çöp kutusu":                "Çöp kutusu",
		"evil\x1b[2J\x1b[Hy? [y/N]": `evil\x1b[2J\x1b[Hy? [y/N]`,
		"line\nbreak":               `line\x0abreak`,
		"bidi\u202ename":            `bidi\u202ename`,
	} {
		if got := Safe(in); got != want {
			t.Errorf("Safe(%q) = %q, want %q", in, got, want)
		}
	}
}
