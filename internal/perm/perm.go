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

// ProtectedDirs are folders macOS guards with privacy prompts (TCC); reading them
// without Full Disk Access can trigger a dialog, so they are read only with --full.
func ProtectedDirs(home string) []string {
	lib := filepath.Join(home, "Library")
	return []string{
		filepath.Join(lib, "Containers"), filepath.Join(lib, "Group Containers"),
		filepath.Join(lib, "Mail"), filepath.Join(lib, "Messages"), filepath.Join(lib, "Safari"),
		filepath.Join(lib, "Calendars"), filepath.Join(lib, "Cookies"), filepath.Join(lib, "HomeKit"),
		filepath.Join(lib, "Application Support", "AddressBook"), filepath.Join(lib, "Application Support", "com.apple.TCC"),
		filepath.Join(lib, "Application Support", "CallHistoryDB"), filepath.Join(lib, "Application Support", "MobileSync"),
		filepath.Join(lib, "Biome"), filepath.Join(lib, "Suggestions"), filepath.Join(lib, "Metadata", "CoreSpotlight"),
		filepath.Join(home, ".Trash"), filepath.Join(home, "Pictures", "Photos Library.photoslibrary"),
	}
}
