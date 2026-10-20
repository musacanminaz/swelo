// Package perm detects what swelo is allowed to do on this Mac.
package perm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FullDiskAccess reports whether this process can read a TCC-protected file.
// Only the TCC database reliably requires Full Disk Access on every macOS version.
func FullDiskAccess(home string) bool {
	f, err := os.Open(filepath.Join(home, "Library", "Application Support", "com.apple.TCC", "TCC.db"))
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// IsAdmin reports whether the current user belongs to the admin group (can sudo).
func IsAdmin() bool {
	out, err := exec.Command("id", "-Gn").Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Fields(string(out)) {
		if g == "admin" {
			return true
		}
	}
	return false
}
