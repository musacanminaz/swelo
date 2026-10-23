package speed

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Stale is a developer process that is most likely forgotten: its project
// is gone, its session ended, or it is a build daemon sitting idle.
type Stale struct {
	Proc
	Reason   string `json:"reason"` // one of the reason constants
	Cwd      string `json:"cwd,omitempty"`
	Children int    `json:"child_processes"` // helpers folded into this row (a browser's renderers)
}

// Reasons (translated by the CLI).
const (
	ReasonTrash   = "project folder is in the Trash"
	ReasonGone    = "project folder no longer exists"
	ReasonDaemon  = "idle build daemon"
	ReasonBrowser = "Playwright browser older than a day"
	ReasonOrphan  = "MCP server whose session ended"
)

// runtimes are the binaries a forgotten dev server or script runs under;
// shells, editors and apps are never offered for stopping.
var runtimes = map[string]bool{"node": true, "python": true, "python3": true, "java": true, "ruby": true, "bun": true,
	"deno": true, "php": true, "dart": true, "flutter": true, "dotnet": true, "tsx": true, "ts-node": true}

// StaleProcesses picks the forgotten ones out of procs: only the current
// user's, never swelo itself. Working directories come from lsof.
func StaleProcesses(run Runner, procs []Proc, home string) []Stale {
	cwd := processCwds(run)
	trash := filepath.Join(home, ".Trash")
	var out []Stale
	for _, p := range procs {
		if !Mine(p) || p.PID == os.Getpid() {
			continue
		}
		exe, rest := SplitArgs(p.Args)
		base := filepath.Base(exe)
		s := Stale{Proc: p, Cwd: cwd[p.PID]}
		switch {
		case runtimes[base] && s.Cwd != "" && (s.Cwd == trash || strings.HasPrefix(s.Cwd, trash+"/")):
			s.Reason = ReasonTrash
		case runtimes[base] && s.Cwd != "" && !exists(s.Cwd):
			s.Reason = ReasonGone
		case base == "java" && p.CPU < 1 && (strings.Contains(rest, "GradleDaemon") || strings.Contains(rest, "KotlinCompileDaemon") || strings.Contains(rest, "kotlin-build-tools")):
			s.Reason = ReasonDaemon
		case p.Age > 24*time.Hour && (strings.Contains(p.Args, "ms-playwright") || strings.Contains(p.Args, "--headless") && browser(exe)):
			s.Reason = ReasonBrowser // Playwright's bundled browsers, MCP profiles, headless runs: never the user's own browser
		case p.PPID == 1 && strings.Contains(p.Args, "-mcp"):
			s.Reason = ReasonOrphan
		default:
			continue
		}
		out = append(out, s)
	}
	return fold(out)
}

// fold keeps one row per process tree: a browser's helpers are summed into
// the browser, which is also the process to stop.
func fold(all []Stale) []Stale {
	idx := map[int]int{}
	for i, s := range all {
		idx[s.PID] = i
	}
	var roots []Stale
	for _, s := range all {
		root := s.PID
		for {
			i, ok := idx[all[idx[root]].PPID]
			if !ok {
				break
			}
			root = all[i].PID
		}
		if root == s.PID {
			roots = append(roots, s)
		}
	}
	for _, s := range all {
		root := s.PID
		for {
			i, ok := idx[all[idx[root]].PPID]
			if !ok {
				break
			}
			root = all[i].PID
		}
		if root != s.PID {
			for j := range roots {
				if roots[j].PID == root {
					roots[j].Mem += s.Mem
					roots[j].Children++
				}
			}
		}
	}
	return roots
}

// processCwds maps pid -> working directory for the current user's processes.
func processCwds(run Runner) map[int]string {
	out, err := run("lsof", "-a", "-d", "cwd", "-u", strconv.Itoa(os.Getuid()), "-Fpn")
	if err != nil && out == "" {
		return nil
	}
	m := map[int]string{}
	pid := 0
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "p"):
			pid, _ = strconv.Atoi(l[1:])
		case strings.HasPrefix(l, "n") && pid > 0:
			m[pid] = l[1:]
		}
	}
	return m
}

// browser: the executable is a browser binary (Chrome, Chromium, Firefox, WebKit).
func browser(exe string) bool {
	l := strings.ToLower(exe)
	return strings.Contains(l, "chrom") || strings.Contains(l, "firefox") || strings.Contains(l, "webkit")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Stop sends SIGTERM to each pid and reports the ones still alive after a
// moment (they are left alone: no SIGKILL without asking again).
func Stop(pids []int) (stillRunning []int, err error) {
	for _, pid := range pids {
		if e := syscall.Kill(pid, syscall.SIGTERM); e != nil && err == nil {
			err = e
		}
	}
	time.Sleep(2 * time.Second)
	for _, pid := range pids {
		if syscall.Kill(pid, 0) == nil {
			stillRunning = append(stillRunning, pid)
		}
	}
	return stillRunning, err
}

// parseEtime reads ps's elapsed time: [[dd-]hh:]mm:ss.
func parseEtime(s string) time.Duration {
	var days int64
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days, _ = strconv.ParseInt(d, 10, 64)
		s = rest
	}
	var secs int64
	for _, part := range strings.Split(s, ":") {
		n, _ := strconv.ParseInt(part, 10, 64)
		secs = secs*60 + n
	}
	return time.Duration(days*86400+secs) * time.Second
}
