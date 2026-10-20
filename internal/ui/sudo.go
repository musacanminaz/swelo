package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/musacanminaz/swelo/internal/i18n"
)

// SudoStep is one action that needs root. It is always shown in full and
// confirmed on its own; --yes never approves it.
type SudoStep struct {
	Action  string   // what happens, e.g. "Remove old JDK temurin-17 (312 MB)"
	Reason  string   // why root is needed
	Command []string // exact command, without "sudo"
	Undo    string   // how to undo, or "" if it cannot be undone
}

// SudoOptions controls how sudo steps are handled.
type SudoOptions struct {
	NoSudo bool                      // skip every sudo step
	Run    func(args []string) error // runs the command; nil = real sudo
}

// ErrSkipped means the step was not run (declined, --no-sudo, or non-admin).
var ErrSkipped = errors.New("sudo step skipped")

// RunSudo shows the step, asks [y/N], and runs it. Afterwards the sudo
// timestamp is invalidated so no root session stays open.
func RunSudo(r *bufio.Reader, w io.Writer, s SudoStep, o SudoOptions) error {
	cmdline := "sudo " + ShellJoin(s.Command)
	fmt.Fprintf(w, "\n %s %s\n\n", Yellow("⚠"), Bold(i18n.T("sudo.title")))
	fmt.Fprintf(w, "   %-12s %s\n", i18n.T("sudo.action"), s.Action)
	fmt.Fprintf(w, "   %-12s %s\n", i18n.T("sudo.reason"), s.Reason)
	fmt.Fprintf(w, "   %-12s %s\n", i18n.T("sudo.command"), cmdline)
	undo := s.Undo
	if undo == "" {
		undo = i18n.T("sudo.noundo")
	}
	fmt.Fprintf(w, "   %-12s %s\n\n", i18n.T("sudo.undo"), undo)

	if o.NoSudo {
		fmt.Fprintf(w, " %s\n", Dim(i18n.T("sudo.skipped.flag")))
		return ErrSkipped
	}
	if !Confirm(r, w, i18n.T("sudo.ask"), false) {
		fmt.Fprintf(w, " %s\n", Dim(i18n.T("sudo.skipped")))
		return ErrSkipped
	}
	run := o.Run
	if run == nil {
		run = realSudo
	}
	return run(s.Command)
}

func realSudo(args []string) error {
	cmd := exec.Command("sudo", append([]string{"--"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	exec.Command("sudo", "-k").Run() // drop cached credentials
	return err
}

// ShellJoin quotes args the way a shell would need them, for display.
func ShellJoin(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t'\"$\\*?;&|<>()") {
			a = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
		out[i] = a
	}
	return strings.Join(out, " ")
}
