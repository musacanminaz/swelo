// Package speed reads what makes a Mac slow: memory pressure, swap, load,
// uptime and the processes behind them. It only reads; acting on a finding
// is a separate, confirmed step.
package speed

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// Proc is one process with its memory footprint (what Activity Monitor
// calls Memory: resident plus compressed) and CPU share.
type Proc struct {
	PID, PPID int
	UID       int
	Mem       int64         `json:"bytes"`
	CPU       float64       `json:"cpu_percent"`
	Args      string        `json:"command"`
	App       string        `json:"app"`
	Age       time.Duration `json:"-"`
}

// App groups the processes of one application.
type App struct {
	Name  string  `json:"name"`
	Mem   int64   `json:"bytes"`
	CPU   float64 `json:"cpu_percent"`
	Procs int     `json:"processes"`
}

// Summary is one reading of the machine.
type Summary struct {
	RAM       int64         `json:"ram_bytes"`
	SwapUsed  int64         `json:"swap_used_bytes"`
	SwapTotal int64         `json:"swap_total_bytes"`
	Load      [3]float64    `json:"load"`
	Cores     int           `json:"cores"`
	Uptime    time.Duration `json:"-"`
	Apps      []App         `json:"apps"` // by memory, largest first
	Procs     []Proc        `json:"-"`    // every process seen
	Errors    []string      `json:"errors,omitempty"`
}

// Snapshot reads memory, swap, load, uptime and every process. CPU shares
// come from top's second sample, so it takes about a second.
func Snapshot(run Runner) Summary {
	var s Summary
	if out, err := run("sysctl", "-n", "hw.memsize", "hw.ncpu", "vm.swapusage", "vm.loadavg", "kern.boottime"); err == nil {
		s.parseSysctl(out, time.Now())
	} else {
		s.Errors = append(s.Errors, "sysctl: "+err.Error())
	}
	top, err := run("top", "-l", "2", "-s", "1", "-o", "mem", "-n", "10000", "-stats", "pid,ppid,mem,cpu")
	if err != nil {
		s.Errors = append(s.Errors, "top: "+err.Error())
	}
	ps, err := run("ps", "-Axo", "pid=,uid=,etime=,args=")
	if err != nil {
		s.Errors = append(s.Errors, "ps: "+err.Error())
	}
	s.Procs = parseProcs(top, ps)
	s.Apps = groupApps(s.Procs)
	return s
}

func (s *Summary) parseSysctl(out string, now time.Time) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 5 {
		return
	}
	s.RAM, _ = strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
	s.Cores, _ = strconv.Atoi(strings.TrimSpace(lines[1]))
	// total = 16384.00M  used = 15700.88M  free = 683.12M  (encrypted)
	for _, kv := range regexp.MustCompile(`(\w+) = ([0-9.]+[KMGT]?)`).FindAllStringSubmatch(lines[2], -1) {
		switch kv[1] {
		case "total":
			s.SwapTotal = parseSize(kv[2])
		case "used":
			s.SwapUsed = parseSize(kv[2])
		}
	}
	for i, f := range strings.Fields(strings.Trim(lines[3], "{} ")) {
		if i < 3 {
			s.Load[i], _ = strconv.ParseFloat(f, 64)
		}
	}
	if m := regexp.MustCompile(`sec = (\d+)`).FindStringSubmatch(lines[4]); m != nil {
		sec, _ := strconv.ParseInt(m[1], 10, 64)
		s.Uptime = now.Sub(time.Unix(sec, 0))
	}
}

// parseProcs joins top's second sample (memory, cpu) with ps (full command).
func parseProcs(top, ps string) []Proc {
	byPID := map[int]*Proc{}
	for _, l := range strings.Split(ps, "\n") {
		f := strings.Fields(l)
		if len(f) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		uid, _ := strconv.Atoi(f[1])
		byPID[pid] = &Proc{PID: pid, UID: uid, Age: parseEtime(f[2]), Args: strings.Join(f[3:], " ")}
	}
	samples := 0
	for _, l := range strings.Split(top, "\n") {
		f := strings.Fields(l)
		if len(f) >= 1 && f[0] == "PID" {
			samples++
			continue
		}
		if samples < 2 || len(f) < 4 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		p := byPID[pid]
		if p == nil {
			continue
		}
		p.PPID, _ = strconv.Atoi(f[1])
		p.Mem = parseSize(strings.TrimRight(f[2], "+-"))
		p.CPU, _ = strconv.ParseFloat(f[3], 64)
	}
	var out []Proc
	for _, p := range byPID {
		p.App = AppName(p.Args)
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mem > out[j].Mem })
	return out
}

func groupApps(procs []Proc) []App {
	idx := map[string]int{}
	var apps []App
	for _, p := range procs {
		i, ok := idx[p.App]
		if !ok {
			i = len(apps)
			idx[p.App] = i
			apps = append(apps, App{Name: p.App})
		}
		apps[i].Mem += p.Mem
		apps[i].CPU += p.CPU
		apps[i].Procs++
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Mem > apps[j].Mem })
	return apps
}

var appRe = regexp.MustCompile(`/([^/]+)\.app/`)

// AppName names the application behind a command line: the first .app
// bundle in its path, or the binary, with a hint for runtimes (java, node,
// python) that would otherwise all look the same. Bundle paths may contain
// spaces ("Android Studio.app"), so the executable is taken up to the first
// flag (" -") when it lives in a bundle, not up to the first space.
func AppName(args string) string {
	exe, rest := SplitArgs(args)
	if m := appRe.FindStringSubmatch(exe); m != nil {
		if hint := runtimeHint(filepath.Base(exe), rest); hint != "" {
			return m[1] + " · " + hint
		}
		return m[1]
	}
	base := filepath.Base(exe)
	switch base {
	case "com.apple.Virtualization.VirtualMachine":
		return "Virtual machine (Docker, Parallels…)"
	case "com.apple.WebKit.WebContent":
		return "Safari / WebKit pages"
	case "claude":
		return "Claude Code"
	}
	if hint := runtimeHint(base, rest); hint != "" {
		return base + " · " + hint
	}
	return base
}

// SplitArgs separates the executable from its arguments. A bundle path may
// contain spaces ("Android Studio.app/…/bin/java"), so when the command
// line starts with a bundle the executable runs up to the first flag.
func SplitArgs(args string) (exe, rest string) {
	head := args
	if i := strings.Index(args, " -"); i >= 0 {
		head = args[:i]
	}
	if strings.Contains(head, ".app/") {
		return head, strings.TrimSpace(args[len(head):])
	}
	exe, rest, _ = strings.Cut(args, " ")
	return exe, rest
}

// runtimeHint says what a java/node/python/ruby process is running.
func runtimeHint(base, rest string) string {
	switch base {
	case "java":
		switch {
		case strings.Contains(rest, "GradleDaemon"):
			return "Gradle daemon"
		case strings.Contains(rest, "KotlinCompileDaemon") || strings.Contains(rest, "kotlin-build-tools"):
			return "Kotlin daemon"
		case strings.Contains(rest, "jdt.ls") || strings.Contains(rest, "language-server"):
			return "language server"
		}
		return "java"
	case "node", "python", "python3", "ruby", "bun", "deno":
		for _, a := range strings.Fields(rest) {
			if strings.HasPrefix(a, "-") {
				continue
			}
			for _, known := range []string{"playwright-mcp", "chrome-devtools-mcp", "vite", "next", "nuxt", "webpack", "esbuild", "tsserver", "typescript-language-server", "jest", "vitest", "nodemon", "ts-node", "tsx", "manage.py", "uvicorn", "gunicorn", "flask", "jupyter"} {
				if strings.Contains(a, known) {
					return known
				}
			}
			return strings.TrimSuffix(filepath.Base(a), filepath.Ext(a))
		}
	}
	return ""
}

var sizeRe = regexp.MustCompile(`^([0-9.]+)([BKMGT]?)`)

// parseSize reads top/sysctl sizes such as "8030M", "1.2G", "683.12M".
func parseSize(s string) int64 {
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseFloat(m[1], 64)
	mult := map[string]float64{"": 1, "B": 1, "K": 1 << 10, "M": 1 << 20, "G": 1 << 30, "T": 1 << 40}[m[2]]
	return int64(n * mult)
}

// Mine reports whether a process belongs to the current user.
func Mine(p Proc) bool { return p.UID == os.Getuid() }
