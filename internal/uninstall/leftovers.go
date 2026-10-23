package uninstall

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/musacanminaz/swelo/internal/clean"
)

// bundleLike matches reverse-DNS names such as com.vendor.App (never Apple's).
var bundleLike = regexp.MustCompile(`^(com|org|net|io|dev|app|co|de|fr|uk|us|ru|se|eu|me|info|biz)\.[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// leftoverAge: a folder still written to recently belongs to something alive.
const leftoverAge = 30 * 24 * time.Hour

// leftovers finds data in ~/Library whose app is gone: entries named by a
// bundle id that no installed app carries, that Spotlight cannot find
// anywhere either, and that nothing has touched for a month. Each id
// becomes one package whose removal moves all of its folders to the Trash.
func leftovers(e *Env) []Package {
	if e.AppDirs == nil {
		return nil
	}
	installed := installedBundleIDs(e)
	lib := filepath.Join(e.Home, "Library")
	folders := []struct{ dir, suffix string }{
		{"Application Support", ""}, {"Caches", ""}, {"Preferences", ".plist"}, {"Saved Application State", ".savedState"},
		{"HTTPStorages", ""}, {"WebKit", ""}, {"Logs", ""}, {"Cookies", ".binarycookies"},
	}
	if e.FDA {
		folders = append(folders, struct{ dir, suffix string }{"Containers", ""})
	}
	paths := map[string][]string{}
	for _, f := range folders {
		entries, _ := os.ReadDir(filepath.Join(lib, f.dir))
		for _, en := range entries {
			name := en.Name()
			if f.suffix != "" {
				if !strings.HasSuffix(name, f.suffix) {
					continue
				}
				name = strings.TrimSuffix(name, f.suffix)
			}
			if !bundleLike.MatchString(name) || strings.HasPrefix(name, "com.apple.") {
				continue
			}
			p := filepath.Join(lib, f.dir, en.Name())
			if e.protectedPath(p) || !directlyUnder(p, lib) {
				continue
			}
			paths[name] = append(paths[name], p)
		}
	}
	var out []Package
	for id, ps := range paths {
		if ownedByInstalled(id, installed) {
			continue
		}
		newest := time.Time{}
		for _, p := range ps {
			if t := mtime(p); t.After(newest) {
				newest = t
			}
		}
		if time.Since(newest) < leftoverAge {
			continue
		}
		// last word: Spotlight, which also knows apps outside the usual folders
		if out, err := e.Run("mdfind", "kMDItemCFBundleIdentifier == '"+id+"'"); err != nil || strings.TrimSpace(out) != "" {
			continue
		}
		sort.Strings(ps)
		p := Package{Provider: "leftover", ID: "leftover:" + id, Name: id, Installed: newest,
			Note: "app is gone", Command: append([]string{"/usr/bin/trash"}, ps...)}
		for _, path := range ps {
			p.Size += clean.DirSize(path)
		}
		out = append(out, p)
	}
	return out
}

// installedBundleIDs reads the bundle id of every app in the app folders
// (one level of subfolders too, e.g. /Applications/Utilities).
func installedBundleIDs(e *Env) map[string]bool {
	ids := map[string]bool{}
	dirs := append([]string{}, e.AppDirs...)
	for _, d := range e.AppDirs {
		entries, _ := os.ReadDir(d)
		for _, en := range entries {
			if en.IsDir() && !strings.HasSuffix(en.Name(), ".app") {
				dirs = append(dirs, filepath.Join(d, en.Name()))
			}
		}
	}
	for _, d := range dirs {
		entries, _ := os.ReadDir(d)
		for _, en := range entries {
			if strings.HasSuffix(en.Name(), ".app") {
				if id := e.bundleID(filepath.Join(d, en.Name())); id != "" {
					ids[strings.ToLower(id)] = true
				}
			}
		}
	}
	return ids
}

// ownedByInstalled: the id is an installed app's, or a helper of one
// (com.vendor.App.helper), or the app of a helper-named entry.
func ownedByInstalled(id string, installed map[string]bool) bool {
	l := strings.ToLower(id)
	if installed[l] {
		return true
	}
	for in := range installed {
		if strings.HasPrefix(l, in+".") || strings.HasPrefix(in, l+".") {
			return true
		}
	}
	return false
}
