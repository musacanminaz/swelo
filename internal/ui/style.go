package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

var color = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return IsTerminal(os.Stdout)
}()

// IsTerminal reports whether f is a real terminal (not /dev/null or a pipe).
func IsTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA)
	return err == nil
}

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

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Pad left-aligns s to n display columns, ignoring color codes and counting
// runes so Turkish letters line up.
func Pad(s string, n int) string {
	w := utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
	if w >= n {
		return s + " "
	}
	return s + strings.Repeat(" ", n-w)
}

// Safe makes an untrusted string (a file name or path) printable: control and
// other non-printable runes become \xNN / \uNNNN, so a crafted name cannot move
// the cursor, recolor the screen or forge a prompt.
func Safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			b.WriteString(`�`)
		case unicode.IsPrint(r):
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	return b.String()
}
