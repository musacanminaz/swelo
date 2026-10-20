package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/rules"
)

// apps lists applications in /Applications and ~/Applications with the
// leftovers they keep in ~/Library (matched by bundle id, or app name for
// Application Support and Logs). Removal moves the app and its leftovers to
// the Trash; Homebrew casks are removed with brew instead.
func apps(e *Env) []Package {
	if e.AppDirs == nil {
		return nil
	}
	casks := caskApps(e)
	var paths []string
	for _, d := range e.AppDirs {
		entries, _ := os.ReadDir(d)
		for _, en := range entries {
			if strings.HasSuffix(en.Name(), ".app") {
				paths = append(paths, filepath.Join(d, en.Name()))
			}
		}
	}
	out := make([]Package, len(paths))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, p := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			out[i] = e.appPackage(p, casks)
			<-sem
		}(i, p)
	}
	wg.Wait()
	var keep []Package
	for _, p := range out {
		if p.ID != "" {
			keep = append(keep, p)
		}
	}
	return keep
}

func (e *Env) appPackage(app string, casks map[string]string) Package {
	id := e.bundleID(app)
	if strings.HasPrefix(id, "com.apple.") && !strings.HasPrefix(id, "com.apple.dt.") {
		return Package{} // Apple's own apps (Xcode, com.apple.dt.*, is listed)
	}
	name := strings.TrimSuffix(filepath.Base(app), ".app")
	p := Package{Provider: "app", ID: "app:" + filepath.Base(app), Name: name, Installed: mtime(app)}
	p.Size = clean.DirSize(app)
	left := e.leftovers(id, name)
	for _, l := range left {
		p.Size += clean.DirSize(l)
	}
	switch {
	case casks[filepath.Base(app)] != "":
		c := casks[filepath.Base(app)]
		p.Command = []string{"brew", "uninstall", "--cask", c}
		p.Note = "installed by Homebrew cask " + c
	case !e.ownedByUser(app):
		p.Note = "owned by root: remove it in Finder or the App Store"
	default:
		p.Command = append([]string{"/usr/bin/trash", app}, left...)
		if len(left) > 0 {
			p.Note = "moves the app and its leftovers to the Trash"
		} else {
			p.Note = "moves the app to the Trash"
		}
	}
	if e.Running != nil && e.Running(app) {
		p.Note = "running — quit it first"
		p.Command = nil
	}
	return p
}

// leftovers are an app's own folders and files in ~/Library. Containers and
// Group Containers are only looked at with Full Disk Access.
func (e *Env) leftovers(id, name string) []string {
	if id == "" {
		return nil
	}
	lib := filepath.Join(e.Home, "Library")
	cands := []string{
		filepath.Join(lib, "Application Support", id),
		filepath.Join(lib, "Application Support", name),
		filepath.Join(lib, "Caches", id),
		filepath.Join(lib, "Caches", name),
		filepath.Join(lib, "HTTPStorages", id),
		filepath.Join(lib, "WebKit", id),
		filepath.Join(lib, "Saved Application State", id+".savedState"),
		filepath.Join(lib, "Preferences", id+".plist"),
		filepath.Join(lib, "Logs", name),
	}
	if e.FDA {
		cands = append(cands, filepath.Join(lib, "Containers", id))
		gc, _ := filepath.Glob(filepath.Join(lib, "Group Containers", "*."+id))
		cands = append(cands, gc...)
	}
	la, _ := filepath.Glob(filepath.Join(lib, "LaunchAgents", id+"*.plist"))
	cands = append(cands, la...)
	var out []string
	seen := map[string]bool{}
	for _, c := range cands {
		if seen[c] || filepath.Base(c) == "" || e.protectedPath(c) {
			continue
		}
		seen[c] = true
		if _, err := os.Lstat(c); err == nil {
			out = append(out, c)
		}
	}
	return out
}

func (e *Env) bundleID(app string) string {
	out, err := e.Run("plutil", "-extract", "CFBundleIdentifier", "raw", filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (e *Env) ownedByUser(app string) bool {
	info, err := os.Stat(app)
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// caskApps maps "DBeaver.app" -> "dbeaver-community" for installed casks.
func caskApps(e *Env) map[string]string {
	m := map[string]string{}
	if !e.Look("brew") {
		return m
	}
	js, err := e.Run("brew", "info", "--cask", "--json=v2", "--installed")
	if err != nil {
		return m
	}
	var d struct {
		Casks []struct {
			Token     string
			Artifacts []map[string]any
		}
	}
	json.Unmarshal([]byte(js), &d)
	for _, c := range d.Casks {
		for _, a := range c.Artifacts {
			list, _ := a["app"].([]any)
			for _, v := range list {
				if s, ok := v.(string); ok {
					m[filepath.Base(s)] = c.Token
				}
			}
		}
	}
	return m
}

// protectedPath keeps swelo's protected folders (e.g. Docker's disk image with
// all volumes) out of an app's leftovers.
func (e *Env) protectedPath(p string) bool {
	for _, pr := range rules.DefaultProtected {
		pr = rules.Expand(pr, e.Home)
		if p == pr || strings.HasPrefix(p, pr+"/") {
			return true
		}
	}
	return false
}
