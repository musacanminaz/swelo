// Package ui holds the screens every command shares: the plan box shown before
// work starts, confirmations, and the sudo step dialog.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/musacanminaz/swelo/internal/i18n"
)

// Plan describes what a command is about to do, shown before it starts.
type Plan struct {
	Title    string
	Scan     []string // locations that will be scanned
	Read     string   // what is read there
	Delete   string   // what will be deleted ("nothing (dry run)" etc.)
	Perm     string   // permissions needed
	Duration string   // rough estimate
}

// ShowPlan prints the plan box.
func ShowPlan(w io.Writer, p Plan) {
	fmt.Fprintf(w, "\n %s — %s\n\n", Bold(p.Title), i18n.T("plan.before"))
	row := func(k, v string) {
		if v != "" {
			fmt.Fprintf(w, "   %-14s %s\n", i18n.T(k), v)
		}
	}
	row("plan.scan", strings.Join(p.Scan, ", "))
	row("plan.read", p.Read)
	row("plan.delete", p.Delete)
	row("plan.perm", p.Perm)
	row("plan.time", p.Duration)
	fmt.Fprintln(w)
}

// Confirm asks a yes/no question. Empty input or EOF returns def.
func Confirm(r *bufio.Reader, w io.Writer, question string, def bool) bool {
	hint := i18n.T("confirm.yn") // [y/N]
	if def {
		hint = i18n.T("confirm.Yn")
	}
	fmt.Fprintf(w, " %s %s ", question, hint)
	line, err := r.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	if err != nil && ans == "" {
		fmt.Fprintln(w)
		return def
	}
	switch ans {
	case "":
		return def
	case "y", "yes", "e", "evet":
		return true
	}
	return false
}

// WaitEnter blocks until the user presses Enter (or input ends).
func WaitEnter(r *bufio.Reader, w io.Writer, msg string) {
	fmt.Fprintf(w, " %s ", Dim(msg))
	r.ReadString('\n')
}
