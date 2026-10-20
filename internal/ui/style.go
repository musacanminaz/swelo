package ui

import (
	"fmt"
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

// Bytes formats a size like "1.4 GB" (decimal units, as Finder shows them).
func Bytes(n int64) string {
	const k = 1000
	switch {
	case n >= k*k*k:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	case n >= k*k:
		return fmt.Sprintf("%.0f MB", float64(n)/(k*k))
	case n >= k:
		return fmt.Sprintf("%.0f KB", float64(n)/k)
	}
	return fmt.Sprintf("%d B", n)
}
