// Package safety is the single gate every deletion in swelo goes through.
package safety

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Guard decides whether a path may be deleted.
type Guard struct {
	Home       string   // user home; never deletable itself
	Roots      []string // deletions must resolve under one of these
	Protected  []string // these paths and everything under them are never deleted
	RecentDays int      // projects touched within this many days are skipped (0 = off)
	LogPath    string   // JSON-lines deletion log ("" = no log)
	Now        func() time.Time
}

// Refusal explains why a path was not deleted.
type Refusal struct {
	Path, Reason string
}

func (r *Refusal) Error() string { return r.Path + ": " + r.Reason }

// protectedNames are never deleted wherever they appear.
var protectedNames = []string{".git", ".ssh", ".gnupg"}

// protectedSuffixes mark user data (dumps, secrets).
var protectedSuffixes = []string{".sql", ".dump", ".sqlite", ".db", ".pem", ".key"}

// Check returns nil if path may be deleted. project, if non-empty, is the
// project directory the path belongs to; it is used for the git and recency checks.
func (g *Guard) Check(path, project string) error {
	refuse := func(reason string) error { return &Refusal{path, reason} }
	if !filepath.IsAbs(path) {
		return refuse("not an absolute path")
	}
	clean := filepath.Clean(path)
	if _, err := os.Lstat(clean); err != nil {
		return refuse("does not exist")
	}
	// Resolve symlinks in the parent so a linked directory cannot escape the roots;
	// the final element itself may be a symlink (removing it only removes the link).
	parent, err := filepath.EvalSymlinks(filepath.Dir(clean))
	if err != nil {
		return refuse("cannot resolve parent")
	}
	real := filepath.Join(parent, filepath.Base(clean))

	if real == filepath.Clean(g.Home) || real == "/" {
		return refuse("home or root directory")
	}
	if !g.underAny(real, g.Roots, false) {
		return refuse("outside allowed roots")
	}
	if g.underAny(real, g.Protected, true) {
		return refuse("protected path")
	}
	for _, r := range g.Roots {
		if real == filepath.Clean(r) {
			return refuse("is an allowed root itself")
		}
	}
	base := filepath.Base(real)
	if strings.HasPrefix(base, ".env") {
		return refuse("environment file")
	}
	for _, n := range protectedNames {
		if base == n || strings.Contains(real, string(filepath.Separator)+n+string(filepath.Separator)) {
			return refuse("inside " + n)
		}
	}
	for _, s := range protectedSuffixes {
		if strings.HasSuffix(strings.ToLower(base), s) {
			return refuse("user data file (" + s + ")")
		}
	}
	if project != "" {
		if rel, err := filepath.Rel(project, real); err == nil && !strings.HasPrefix(rel, "..") {
			if gitTracked(project, rel) {
				return refuse("tracked by git")
			}
		}
		if g.RecentDays > 0 && g.recent(project, base) {
			return refuse(fmt.Sprintf("project changed in the last %d days", g.RecentDays))
		}
	}
	return nil
}

// Remove deletes path if Check allows it and records it in the log.
// With dryRun it only checks.
func (g *Guard) Remove(path, project string, size int64, rule string, dryRun bool) error {
	if err := g.Check(path, project); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return g.log(path, size, rule)
}

func (g *Guard) underAny(p string, list []string, inclusive bool) bool {
	for _, r := range list {
		r = filepath.Clean(r)
		if rr, err := filepath.EvalSymlinks(r); err == nil {
			r = rr
		}
		if (inclusive && p == r) || strings.HasPrefix(p, r+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// recent reports whether anything directly in project (except skip), or its git
// index, was modified within RecentDays.
func (g *Guard) recent(project, skip string) bool {
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	cutoff := now().AddDate(0, 0, -g.RecentDays)
	entries, err := os.ReadDir(project)
	if err != nil {
		return true // unreadable: be conservative
	}
	for _, e := range entries {
		if e.Name() == skip {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(cutoff) {
			return true
		}
	}
	if info, err := os.Stat(filepath.Join(project, ".git", "index")); err == nil && info.ModTime().After(cutoff) {
		return true
	}
	return false
}

func gitTracked(dir, rel string) bool {
	out, err := exec.Command("git", "-C", dir, "ls-files", "--", rel).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func (g *Guard) log(path string, size int64, rule string) error {
	if g.LogPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(g.LogPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(g.LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(map[string]any{
		"time": time.Now().Format(time.RFC3339), "path": path, "bytes": size, "rule": rule,
	})
}

// IsRefusal reports whether err is a safety refusal (as opposed to an I/O error).
func IsRefusal(err error) bool {
	var r *Refusal
	return errors.As(err, &r)
}
