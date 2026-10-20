package ui

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

// Text styles; plain text when stdout is not a terminal or NO_COLOR is set.
func Bold(s string) string   { return ansi("1", s) }
func Dim(s string) string    { return ansi("2", s) }
func Green(s string) string  { return ansi("32", s) }
func Yellow(s string) string { return ansi("33", s) }
func Red(s string) string    { return ansi("31", s) }

// Indent prefixes every line of s with pre.
func Indent(s, pre string) string {
	return pre + strings.ReplaceAll(s, "\n", "\n"+pre)
}
