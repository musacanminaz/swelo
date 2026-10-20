package cli

import (
	"os"
	"strings"
)

var color = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

func ansi(code, s string) string {
	if !color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func bold(s string) string   { return ansi("1", s) }
func dim(s string) string    { return ansi("2", s) }
func green(s string) string  { return ansi("32", s) }
func yellow(s string) string { return ansi("33", s) }

func indent(s, pre string) string {
	return pre + strings.ReplaceAll(s, "\n", "\n"+pre)
}
