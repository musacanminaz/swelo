// Package perm detects what swelo is allowed to do on this Mac.
package perm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
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

// sudoSession is set while the user has granted admin rights from the menu.
var sudoSession atomic.Bool

// SetSudoSession records whether sudo credentials are cached for this terminal.
func SetSudoSession(on bool) { sudoSession.Store(on) }

// Isolate starts cmd in a new session without a controlling terminal while
// admin rights are granted. sudo caches credentials per terminal, so tools
// swelo runs (brew, npm, docker, git and their scripts) cannot use them
// silently; a sudo inside them fails instead of running as root.
func Isolate(cmd *exec.Cmd) *exec.Cmd {
	if sudoSession.Load() {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Setsid = true
	}
	return cmd
}

// TerminalNeighbours lists processes on this process's terminal other than
// itself and its ancestors (the shell, the terminal app): background jobs or
// leftovers that could also use sudo credentials cached for this terminal.
func TerminalNeighbours() []string {
	out, err := exec.Command("ps", "-ax", "-o", "pid=,ppid=,tty=,comm=").Output()
	if err != nil {
		return nil
	}
	type proc struct {
		ppid      int
		tty, name string
	}
	procs := map[int]proc{}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		ppid, _ := strconv.Atoi(f[1])
		procs[pid] = proc{ppid, f[2], strings.Join(f[3:], " ")}
	}
	self := os.Getpid()
	tty := procs[self].tty
	if tty == "" || tty == "??" {
		return nil
	}
	mine := map[int]bool{}
	for p := self; p > 1 && !mine[p]; p = procs[p].ppid {
		mine[p] = true
	}
	var list []string
	for pid, p := range procs {
		if p.tty == tty && !mine[pid] && !(p.ppid == self && filepath.Base(p.name) == "ps") {
			list = append(list, fmt.Sprintf("%d %s", pid, filepath.Base(p.name)))
		}
	}
	sort.Strings(list)
	return list
}
