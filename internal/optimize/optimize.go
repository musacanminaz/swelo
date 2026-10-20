// Package optimize holds maintenance tasks whose effect can be measured:
// Time Machine exclusions for regenerable folders, git gc on large repos,
// and Homebrew autoremove. Each task is planned first and applied only on request.
package optimize

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/ui"
)

// Task is one planned maintenance job.
type Task struct {
	ID    string
	Items []string // paths it acts on (for display)
	Size  int64    // bytes involved (excluded from backups, or .git size)
	Note  string   // why it is skipped, or extra info
	Ready bool     // can be applied
	apply func() (string, error)
}

// Apply runs the task and returns a short result.
func (t Task) Apply() (string, error) {
	if !t.Ready || t.apply == nil {
		return "", nil
	}
	return t.apply()
}

// Runner executes commands; tests replace it.
type Runner func(argv ...string) (string, error)

// Real runs commands with a timeout.
func Real(argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr = &buf, &buf
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	err := cmd.Run()
	return buf.String(), err
}

// TimeMachine excludes regenerable folders from backups with sticky
// exclusions (`tmutil addexclusion`, no sudo; undo: `tmutil removeexclusion`).
func TimeMachine(run Runner, dirs []string) Task {
	t := Task{ID: "timemachine"}
	if out, _ := run("tmutil", "destinationinfo"); strings.Contains(out, "No destinations") {
		t.Note = "Time Machine is not set up"
		return t
	}
	if len(dirs) == 0 {
		return t
	}
	var safe []string
	for _, d := range dirs {
		if !strings.ContainsFunc(d, unicode.IsControl) {
			safe = append(safe, d)
		}
	}
	dirs = safe
	if len(dirs) == 0 {
		return t
	}
	out, err := run(append([]string{"tmutil", "isexcluded"}, dirs...)...)
	if err != nil {
		t.Note = "tmutil isexcluded failed"
		return t
	}
	// trust only paths we passed in; tmutil's output is matched against them
	asked := map[string]bool{}
	for _, d := range dirs {
		asked[d] = true
	}
	for _, l := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(strings.TrimSpace(l), "[Included]"); ok {
			p = strings.TrimSpace(p)
			if asked[p] {
				t.Items = append(t.Items, p)
				t.Size += clean.DirSize(p)
				delete(asked, p)
			}
		}
	}
	t.Ready = len(t.Items) > 0
	items := t.Items
	t.apply = func() (string, error) {
		return run(append([]string{"tmutil", "addexclusion"}, items...)...)
	}
	return t
}

// Git runs `git gc` and `git worktree prune` on repositories whose .git is at
// least minSize, skipping repos with a lock file or a busy project.
func Git(run Runner, repos []string, minSize int64, busy func(string) bool) Task {
	t := Task{ID: "git-gc"}
	var picked []string
	for _, r := range repos {
		g := filepath.Join(r, ".git")
		if _, err := os.Stat(filepath.Join(g, "index.lock")); err == nil {
			continue // an operation is in progress
		}
		if busy != nil && busy(r) {
			continue
		}
		if !ownedByUser(g) || unsafeConfig(run, r) {
			continue // another user's repo, or its config can run commands
		}
		if size := clean.DirSize(g); size >= minSize {
			picked = append(picked, r)
			t.Items = append(t.Items, r)
			t.Size += size
		}
	}
	t.Ready = len(picked) > 0
	t.apply = func() (string, error) {
		var before, after int64
		for _, r := range picked {
			before += clean.DirSize(filepath.Join(r, ".git"))
			// repo-controlled executables stay disabled (see safety.gitTracked)
			base := []string{"git", "-c", "core.fsmonitor=", "-c", "core.hooksPath=/dev/null",
				"-c", "gc.recentObjectsHook=", "-c", "gc.autoDetach=false", "-C", r}
			if out, err := run(append(base, "gc", "--quiet")...); err != nil {
				return out, err
			}
			run(append(base, "worktree", "prune")...)
			after += clean.DirSize(filepath.Join(r, ".git"))
		}
		return ui.Bytes(before) + " → " + ui.Bytes(after), nil
	}
	return t
}

// BrewAutoremove removes formulae that were only installed as dependencies
// and are no longer needed.
func BrewAutoremove(run Runner, have func(string) bool) Task {
	t := Task{ID: "brew-autoremove"}
	if !have("brew") {
		t.Note = "brew not installed"
		return t
	}
	out, err := run("brew", "autoremove", "--dry-run")
	if err != nil {
		t.Note = "brew autoremove --dry-run failed"
		return t
	}
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "==>") {
			t.Items = append(t.Items, l)
		}
	}
	t.Ready = len(t.Items) > 0
	t.apply = func() (string, error) { return run("brew", "autoremove") }
	return t
}

// FindRepos lists git repositories under roots (not inside regenerable folders).
func FindRepos(roots []string, skip map[string]bool) []string {
	var repos []string
	for _, root := range roots {
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			if d.Name() == ".git" {
				repos = append(repos, filepath.Dir(p))
				return filepath.SkipDir
			}
			return nil
		})
	}
	return repos
}

// commandKeys are git config keys that can make git run a program.
var commandKeys = regexp.MustCompile(`(?i)(hook|command|cmd|program|fsmonitor|editor|pager|askpass|^include\.|^includeif\.|^filter\.|^alias\.|^credential\.)`)

// unsafeConfig reports whether the repository's own config sets any key that
// could execute a command (or pulls in other config files).
func unsafeConfig(run Runner, repo string) bool {
	out, err := run("git", "-C", repo, "config", "--local", "--name-only", "--list")
	if err != nil {
		return true
	}
	for _, k := range strings.Fields(out) {
		if commandKeys.MatchString(k) {
			return true
		}
	}
	return false
}

func ownedByUser(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
