// Package startup reads what macOS starts for the user at login or boot
// (launchd agents and daemons outside Apple's own), finds the app behind
// each one, and turns the chosen ones off reversibly.
package startup

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Runner executes commands; tests replace it.
type Runner func(argv ...string) (string, error)

// Real runs commands with a timeout.
func Real(argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = &buf
	err := cmd.Run()
	return buf.String(), err
}

// Kinds of startup items, by where the plist lives.
const (
	UserAgent   = "user agent"   // ~/Library/LaunchAgents: this user's, no sudo
	GlobalAgent = "global agent" // /Library/LaunchAgents: every user's login; turned off for this user without sudo
	Daemon      = "daemon"       // /Library/LaunchDaemons: at boot as root; sudo
)

// Item is one launchd job.
type Item struct {
	Label     string `json:"label"`
	File      string `json:"file"`
	Kind      string `json:"kind"`
	Program   string `json:"program,omitempty"`
	App       string `json:"app,omitempty"` // bundle name when the program lives in one
	RunAtLoad bool   `json:"run_at_load"`
	KeepAlive bool   `json:"keep_alive"`
	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	Orphan    bool   `json:"orphan"`  // program (or its app) is gone, or the plist is empty
	Updater   bool   `json:"updater"` // an auto-updater or telemetry helper
	Disabled  bool   `json:"disabled"`
	Empty     bool   `json:"empty"` // an empty plist left behind by an uninstaller
}

// Scan lists every non-Apple launchd job in the three folders.
func Scan(run Runner, home string, uid int) []Item {
	running := loaded(run)
	disabled := disabledSet(run, uid)
	var items []Item
	for _, d := range []struct{ dir, kind string }{
		{filepath.Join(home, "Library", "LaunchAgents"), UserAgent},
		{"/Library/LaunchAgents", GlobalAgent},
		{"/Library/LaunchDaemons", Daemon},
	} {
		entries, _ := os.ReadDir(d.dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") || strings.HasPrefix(e.Name(), "com.apple.") {
				continue
			}
			it := parse(run, filepath.Join(d.dir, e.Name()), d.kind)
			if strings.HasPrefix(it.Label, "com.apple.") {
				continue
			}
			if pid, ok := running[it.Label]; ok {
				it.Running, it.PID = pid > 0, pid
			}
			it.Disabled = disabled[it.Label]
			items = append(items, it)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return kindOrder(items[i].Kind) < kindOrder(items[j].Kind)
		}
		return items[i].Label < items[j].Label
	})
	return items
}

func kindOrder(k string) int {
	switch k {
	case UserAgent:
		return 0
	case GlobalAgent:
		return 1
	}
	return 2
}

// parse reads one plist through plutil (binary or XML) and works out the
// program and app behind it.
func parse(run Runner, file, kind string) Item {
	it := Item{Label: strings.TrimSuffix(filepath.Base(file), ".plist"), File: file, Kind: kind}
	updater := func(label string) bool {
		l := strings.ToLower(label)
		for _, k := range []string{"update", "keystone", "upgrader", "syncreporter", "ccxprocess", "crashreporter", "telemetry"} {
			if strings.Contains(l, k) {
				return true
			}
		}
		return false
	}
	it.Updater = updater(it.Label)
	out, err := run("plutil", "-convert", "json", "-o", "-", file)
	if err != nil {
		return it
	}
	var p struct {
		Label            string
		Program          string
		ProgramArguments []string
		RunAtLoad        bool
		KeepAlive        any
		Disabled         bool
	}
	if json.Unmarshal([]byte(out), &p) != nil || strings.TrimSpace(out) == "{}" {
		it.Empty, it.Orphan = true, true
		return it
	}
	if p.Label != "" {
		it.Label = p.Label
	}
	it.RunAtLoad = p.RunAtLoad
	it.KeepAlive = p.KeepAlive != nil && p.KeepAlive != false
	it.Program = p.Program
	if it.Program == "" && len(p.ProgramArguments) > 0 {
		it.Program = p.ProgramArguments[0]
	}
	it.Disabled = p.Disabled
	if i := strings.Index(it.Program, ".app/"); i >= 0 {
		bundle := it.Program[:i+4]
		it.App = strings.TrimSuffix(filepath.Base(bundle), ".app")
		it.Orphan = !exists(bundle)
	} else if it.Program != "" {
		it.Orphan = !exists(it.Program)
	}
	it.Updater = updater(it.Label)
	return it
}

// loaded maps label -> pid (0 when loaded but not running) for the user's
// launchd domain.
func loaded(run Runner) map[string]int {
	m := map[string]int{}
	out, err := run("launchctl", "list")
	if err != nil {
		return m
	}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || f[2] == "Label" {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		m[f[2]] = pid
	}
	return m
}

// disabledSet reads `launchctl print-disabled gui/<uid>`.
func disabledSet(run Runner, uid int) map[string]bool {
	m := map[string]bool{}
	out, err := run("launchctl", "print-disabled", "gui/"+strconv.Itoa(uid))
	if err != nil {
		return m
	}
	for _, l := range strings.Split(out, "\n") {
		if label, state, ok := strings.Cut(strings.TrimSpace(l), " => "); ok && state == "disabled" {
			m[strings.Trim(label, `"`)] = true
		}
	}
	return m
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Commands returns what turns an item off and what turns it back on. Agents
// are disabled for this user only; a daemon needs root.
func Commands(it Item, uid int) (off [][]string, undo string, sudo bool) {
	u := strconv.Itoa(uid)
	switch it.Kind {
	case Daemon:
		return [][]string{{"launchctl", "bootout", "system/" + it.Label}, {"launchctl", "disable", "system/" + it.Label}},
			"sudo launchctl enable system/" + it.Label + " && sudo launchctl bootstrap system " + it.File, true
	default:
		return [][]string{{"launchctl", "bootout", "gui/" + u + "/" + it.Label}, {"launchctl", "disable", "gui/" + u + "/" + it.Label}},
			"launchctl enable gui/" + u + "/" + it.Label + " && launchctl bootstrap gui/" + u + " " + it.File, false
	}
}
