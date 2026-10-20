// Package clean turns rules into concrete items and deletes approved ones
// through the safety gate.
package clean

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/safety"
)

// Item is one deletable path.
type Item struct {
	Path    string
	Project string // project dir for artifact items ("" otherwise)
	Size    int64
	Skip    string // non-empty: why it will not be deleted
}

// Result is what one rule found.
type Result struct {
	Rule  rules.Rule
	Items []Item // deletable
	Skips []Item // found but refused
	Size  int64  // total of Items, or measured size for command rules
	Note  string // e.g. "docker is not running"
	Show  string // command rule dry-run output
}

// Engine scans and applies rules.
type Engine struct {
	FDA      bool // Full Disk Access: TCC-protected folders may be read
	Home     string
	Cfg      rules.Config
	Guard    *safety.Guard
	Rules    []rules.Rule
	procs    string // `ps` output, for running checks
	excludes []*regexp.Regexp
	projects []string // expanded project roots
	cwds     []string // working directories of running processes

	// OnProgress, if set, hears what Scan is doing; calls are serialized.
	OnProgress func(Progress)
	progMu     sync.Mutex
}

// Progress is one step of a Scan. Stage is "rule" (Detail: rule id), "walk"
// (Detail: the directory being read, N: directories read, Found: matches),
// "measure" or "check" (Detail: the item, N of Total done).
type Progress struct {
	Stage, Detail   string
	N, Total, Found int
}

func (e *Engine) report(p Progress) {
	if e.OnProgress == nil {
		return
	}
	e.progMu.Lock()
	e.OnProgress(p)
	e.progMu.Unlock()
}

// New builds an engine and its safety guard from rules and config.
func New(home string, rs []rules.Rule, cfg rules.Config) *Engine {
	e := &Engine{Home: home, Cfg: cfg, Rules: rs}
	var roots, protected, names []string
	e.projects = rules.ProjectRoots(cfg, home)
	roots = append(roots, e.projects...)
	for _, p := range cfg.Protected {
		protected = append(protected, rules.Expand(p, home))
	}
	for _, r := range rs {
		names = append(names, r.Names...)
		if r.Kind != "path" || r.Risk == rules.Report {
			continue // report rules never delete, so they add no roots
		}
		for _, p := range r.Paths {
			matches, _ := filepath.Glob(rules.Expand(p, home))
			for _, m := range matches {
				root := m
				if !r.Children {
					root = filepath.Dir(m)
				}
				if root != home && root != "/" && trustedRoot(root, home) {
					roots = append(roots, root)
				}
			}
		}
	}
	for _, x := range cfg.Exclude {
		e.excludes = append(e.excludes, globRegexp(rules.Expand(x, home)))
	}
	e.Guard = &safety.Guard{
		Home: home, Roots: roots, Protected: protected, RecentDays: *cfg.RecentDays,
		SkipNames: names, LogPath: filepath.Join(home, ".local", "state", "swelo", "history.jsonl"),
	}
	if out, err := exec.Command("ps", "-axo", "command=").Output(); err == nil {
		e.procs = string(out)
	}
	e.cwds = processCwds()
	return e
}

// globRegexp turns a glob where * also matches "/" into a regexp.
func globRegexp(g string) *regexp.Regexp {
	q := regexp.QuoteMeta(g)
	q = strings.ReplaceAll(q, `\*`, ".*")
	q = strings.ReplaceAll(q, `\?`, ".")
	return regexp.MustCompile("^" + q + "$")
}

func (e *Engine) excluded(p string) bool {
	for _, re := range e.excludes {
		if re.MatchString(p) {
			return true
		}
	}
	return false
}

// RunningMatch returns the first running-process pattern of r that is active.
func (e *Engine) RunningMatch(r rules.Rule) string {
	for _, pat := range r.Running {
		if strings.Contains(e.procs, pat) {
			return pat
		}
	}
	return ""
}

// Scan evaluates every enabled rule. Artifact rules share one walk of the project roots.
func (e *Engine) Scan() []Result {
	var artifact []int
	results := make([]Result, len(e.Rules))
	for i, r := range e.Rules {
		results[i].Rule = r
		if r.Disabled {
			continue
		}
		if r.Kind != "artifact" {
			e.report(Progress{Stage: "rule", Detail: r.ID, N: i + 1, Total: len(e.Rules)})
		}
		switch r.Kind {
		case "artifact":
			artifact = append(artifact, i)
		case "path":
			results[i].Items = e.pathItems(r)
		case "command":
			e.command(&results[i])
		}
	}
	e.walkArtifacts(results, artifact)

	// sizes in parallel, then safety checks
	var all []*Item
	for i := range results {
		for j := range results[i].Items {
			all = append(all, &results[i].Items[j])
		}
	}
	var measured atomic.Int64
	measure(all, func(it *Item) {
		e.report(Progress{Stage: "measure", Detail: it.Path, N: int(measured.Add(1)), Total: len(all)})
	})
	checked := 0
	for i := range results {
		res := &results[i]
		if pat := e.RunningMatch(res.Rule); pat != "" && len(res.Items) > 0 {
			res.Note = "running: " + pat
			for _, it := range res.Items {
				it.Skip = "app is running (" + pat + ")"
				res.Skips = append(res.Skips, it)
			}
			res.Items = nil
		}
		var keep []Item
		for _, it := range res.Items {
			checked++
			e.report(Progress{Stage: "check", Detail: it.Path, N: checked, Total: len(all)})
			if it.Skip == "" && e.excluded(it.Path) {
				it.Skip = "excluded in config"
			}
			if it.Skip == "" && it.Project != "" && e.busy(it.Project) {
				it.Skip = "a process is running in this project"
			}
			if it.Skip == "" && res.Rule.Risk != rules.Report {
				if err := e.Guard.Check(it.Path, it.Project); err != nil {
					if r, ok := err.(*safety.Refusal); ok {
						it.Skip = r.Reason
					} else {
						it.Skip = err.Error()
					}
				}
			}
			if it.Skip != "" {
				res.Skips = append(res.Skips, it)
				continue
			}
			keep = append(keep, it)
			res.Size += it.Size
		}
		res.Items = keep
		sort.Slice(res.Items, func(a, b int) bool { return res.Items[a].Size > res.Items[b].Size })
	}
	return results
}

func (e *Engine) walkArtifacts(results []Result, idx []int) {
	if len(idx) == 0 {
		return
	}
	byName := map[string][]int{}
	for _, i := range idx {
		for _, n := range results[i].Rule.Names {
			byName[n] = append(byName[n], i)
		}
	}
	seen := map[string]bool{}
	dirs, found := 0, 0
	for _, root := range e.projects {
		real, err := filepath.EvalSymlinks(root)
		if err != nil || seen[real] {
			continue
		}
		seen[real] = true
		filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			dirs++
			if dirs%32 == 1 {
				e.report(Progress{Stage: "walk", Detail: p, N: dirs, Found: found})
			}
			name := d.Name()
			if name == ".git" || (filepath.Dir(p) == e.Home && (name == "Library" || name == ".Trash")) {
				return filepath.SkipDir
			}
			for _, i := range byName[name] {
				parent := filepath.Dir(p)
				if hasMarker(parent, results[i].Rule.Markers) {
					// excluded matches are still listed so the user sees them skipped
					results[i].Items = append(results[i].Items, Item{Path: p, Project: parent})
					found++
					e.report(Progress{Stage: "walk", Detail: p, N: dirs, Found: found})
					return filepath.SkipDir
				}
			}
			// hidden folders (.vscode, .npm…) hold tools' own data, and go/pkg is
			// Go's module cache: their package.json files are not projects
			if e.excluded(p) || (strings.HasPrefix(name, ".") && p != real) || p == filepath.Join(e.Home, "go", "pkg") {
				return filepath.SkipDir
			}
			return nil
		})
	}
	e.report(Progress{Stage: "walk", N: dirs, Found: found})
}

func hasMarker(dir string, markers []string) bool {
	if len(markers) == 0 {
		return true
	}
	for _, m := range markers {
		if ms, _ := filepath.Glob(filepath.Join(dir, m)); len(ms) > 0 {
			return true
		}
	}
	return false
}

func (e *Engine) pathItems(r rules.Rule) []Item {
	var items []Item
	for _, p := range r.Paths {
		pattern := rules.Expand(p, e.Home)
		if !e.FDA && e.tccProtected(pattern) {
			continue // reading it could pop up a macOS privacy dialog
		}
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			if !r.Children {
				items = append(items, Item{Path: m})
				continue
			}
			items = append(items, children(m, r)...)
		}
	}
	return items
}

func children(dir string, r rules.Rule) []Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -r.OlderThanDays)
	var names []string
	var items []Item
	for _, en := range entries {
		n := en.Name()
		if r.NameGlob != "" {
			if ok, _ := filepath.Match(r.NameGlob, n); !ok {
				continue
			}
		}
		p := filepath.Join(dir, n)
		it := Item{Path: p}
		if r.OlderThanDays > 0 {
			if info, err := en.Info(); err != nil || info.ModTime().After(cutoff) {
				continue
			}
		}
		if r.SkipIfChild != "" && lockHeld(filepath.Join(p, r.SkipIfChild)) {
			it.Skip = "in use (" + r.SkipIfChild + ")"
		}
		names = append(names, n)
		items = append(items, it)
	}
	if r.KeepLatest > 0 {
		keep := Latest(names, r.KeepLatest)
		for i := range items {
			if keep[names[i]] {
				items[i].Skip = "newest version, kept"
			}
		}
	}
	return items
}

// Latest returns the newest n names per prefix, where "chromium-1208" has
// prefix "chromium" and version 1208, and "34.0.0" has an empty prefix.
func Latest(names []string, n int) map[string]bool {
	groups := map[string][]string{}
	for _, name := range names {
		prefix, _ := splitVersion(name)
		groups[prefix] = append(groups[prefix], name)
	}
	keep := map[string]bool{}
	for _, g := range groups {
		sort.Slice(g, func(a, b int) bool {
			_, va := splitVersion(g[a])
			_, vb := splitVersion(g[b])
			return versionLess(vb, va)
		})
		for i := 0; i < n && i < len(g); i++ {
			keep[g[i]] = true
		}
	}
	return keep
}

func splitVersion(name string) (prefix, version string) {
	if i := strings.LastIndex(name, "-"); i >= 0 && i+1 < len(name) && name[i+1] >= '0' && name[i+1] <= '9' {
		return name[:i], name[i+1:]
	}
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return "", name
	}
	return name, ""
}

func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, ea := strconv.Atoi(pa[i])
		nb, eb := strconv.Atoi(pb[i])
		if ea == nil && eb == nil && na != nb {
			return na < nb
		}
		if (ea != nil || eb != nil) && pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return len(pa) < len(pb)
}

func (e *Engine) command(res *Result) {
	r := res.Rule
	if r.Requires != "" {
		if _, err := exec.LookPath(r.Requires); err != nil {
			res.Note = r.Requires + " not installed"
			return
		}
	}
	if len(r.Check) > 0 {
		if _, err := run(r.Check, 15*time.Second); err != nil {
			res.Note = strings.Join(r.Check, " ") + " failed (not running?)"
			return
		}
	}
	if len(r.Show) > 0 {
		out, _ := run(r.Show, 30*time.Second)
		res.Show = strings.TrimRight(out, "\n")
	}
	if len(r.SizeTextCmd) > 0 {
		if out, err := run(r.SizeTextCmd, 30*time.Second); err == nil {
			res.Size = ParseSize(out)
		}
	}
	if len(r.SizeCmd) > 0 {
		if out, err := run(r.SizeCmd, 15*time.Second); err == nil {
			if p := strings.TrimSpace(out); p != "" {
				res.Size = DirSize(p)
				res.Show = p
			}
		}
	}
}

// RunCommands executes a command rule; returns combined output.
func RunCommands(r rules.Rule) (string, error) {
	var all []string
	for _, c := range r.Run {
		out, err := run(c, 30*time.Minute)
		all = append(all, strings.TrimSpace(out))
		if err != nil {
			return strings.Join(all, "\n"), err
		}
	}
	return strings.Join(all, "\n"), nil
}

func run(argv []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.String(), err
}

func measure(items []*Item, done func(*Item)) {
	ch := make(chan *Item)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				it.Size = DirSize(it.Path)
				done(it)
			}
		}()
	}
	for _, it := range items {
		ch <- it
	}
	close(ch)
	wg.Wait()
}

// DirSize returns allocated bytes under p, counting hard links once (like du).
func DirSize(p string) int64 {
	var total int64
	seen := map[uint64]bool{}
	add := func(info fs.FileInfo) {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			if st.Nlink > 1 {
				if seen[st.Ino] {
					return
				}
				seen[st.Ino] = true
			}
			total += st.Blocks * 512
		}
	}
	info, err := os.Lstat(p)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		add(info)
		return total
	}
	filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if info, err := d.Info(); err == nil {
			add(info)
		}
		return nil
	})
	return total
}

// FreeBytes returns available bytes on the volume holding path.
func FreeBytes(path string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}

// Selected reports whether a result takes part in --apply.
func Selected(r Result, risky bool) bool {
	switch {
	case r.Rule.Disabled || r.Rule.Risk == rules.Report:
		return false
	case r.Rule.Risk == rules.Risky:
		return risky
	}
	return true
}

// Apply deletes the items of every selected result through the safety gate
// and runs command rules. report is called after each item or command.
func (e *Engine) Apply(results []Result, risky bool, report func(r Result, path string, size int64, err error)) (freed int64) {
	for _, res := range results {
		if !Selected(res, risky) {
			continue
		}
		if res.Rule.Kind == "command" {
			if res.Note != "" {
				continue
			}
			out, err := RunCommands(res.Rule)
			report(res, out, 0, err)
			continue
		}
		for _, it := range res.Items {
			err := e.Guard.Remove(it.Path, it.Project, it.Size, res.Rule.ID, false)
			if err == nil {
				freed += it.Size
			}
			report(res, it.Path, it.Size, err)
		}
	}
	return freed
}

// processCwds lists the working directories of the user's running processes.
func processCwds() []string {
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-u", strconv.Itoa(os.Getuid()), "-Fn").Output()
	if err != nil && len(out) == 0 {
		return nil
	}
	var cwds []string
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "n/") && l != "n/" {
			cwds = append(cwds, l[1:])
		}
	}
	return cwds
}

// Busy reports whether a running process has its working directory inside dir.
func (e *Engine) Busy(dir string) bool { return e.busy(dir) }

// busy reports whether a process works inside dir.
func (e *Engine) busy(dir string) bool {
	for _, c := range e.cwds {
		if c == dir || strings.HasPrefix(c, dir+"/") {
			return true
		}
	}
	return false
}

// lockHeld reports whether a lock file exists and, for Chromium's
// "host-PID" SingletonLock symlink, whether that process is still alive.
func lockHeld(p string) bool {
	target, err := os.Readlink(p)
	if err != nil {
		_, statErr := os.Lstat(p)
		return statErr == nil // a regular lock file: assume held
	}
	i := strings.LastIndex(target, "-")
	pid, err := strconv.Atoi(target[i+1:])
	if i < 0 || err != nil || pid <= 0 {
		return true
	}
	return syscall.Kill(pid, 0) == nil || syscall.Kill(pid, 0) == syscall.EPERM
}

// trustedRoot rejects roots outside home (e.g. /private/tmp/claude-$UID) unless
// they are real directories owned by this user and writable only by them, so
// another user cannot plant a symlink or folder there for swelo to delete.
func trustedRoot(root, home string) bool {
	if strings.HasPrefix(root, home+"/") {
		return true
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

// ParseSize reads the first size in s, e.g. "1.2GB (100%)" or "512kB"; 0 if none.
func ParseSize(s string) int64 {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	mult := map[string]float64{"": 1, "B": 1, "K": 1e3, "M": 1e6, "G": 1e9, "T": 1e12}[strings.ToUpper(m[2])]
	return int64(n * mult)
}

var sizeRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*([kKMGT]?)i?B`)

func (e *Engine) tccProtected(p string) bool {
	for _, d := range perm.ProtectedDirs(e.Home) {
		if p == d || strings.HasPrefix(p, d+"/") {
			return true
		}
	}
	return false
}
