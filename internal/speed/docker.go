package speed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Docker is what Docker Desktop takes from the Mac and what runs in it.
type Docker struct {
	Installed     bool        `json:"installed"`
	Running       bool        `json:"running"`
	MemoryBytes   int64       `json:"memory_bytes,omitempty"` // VM limit from Docker Desktop's settings
	CPUs          int         `json:"cpus,omitempty"`
	ResourceSaver bool        `json:"resource_saver"`
	SettingsRead  bool        `json:"settings_read"` // needs Full Disk Access (Group Containers)
	Containers    []Container `json:"containers"`
}

// Container is a running container with its current load.
type Container struct {
	Name       string  `json:"name"`
	Up         string  `json:"up"` // "Up 4 days (healthy)"
	CPUPercent float64 `json:"cpu_percent"`
	MemBytes   int64   `json:"memory_bytes"`
}

// Idle: no CPU to speak of right now.
func (c Container) Idle() bool { return c.CPUPercent < 0.5 }

// DockerInfo reads Docker Desktop's resource settings (only with Full Disk
// Access, the file is in Group Containers) and `docker ps` + `docker stats`.
func DockerInfo(run Runner, look func(string) bool, home string, fda bool) Docker {
	var d Docker
	if !look("docker") {
		return d
	}
	d.Installed = true
	if fda {
		if b, err := os.ReadFile(filepath.Join(home, "Library", "Group Containers", "group.com.docker", "settings-store.json")); err == nil {
			var s struct {
				MemoryMiB        int64
				Cpus             int
				UseResourceSaver bool
			}
			if json.Unmarshal(b, &s) == nil {
				d.SettingsRead = true
				d.MemoryBytes, d.CPUs, d.ResourceSaver = s.MemoryMiB<<20, s.Cpus, s.UseResourceSaver
			}
		}
	}
	if _, err := run("docker", "info"); err != nil {
		return d
	}
	d.Running = true
	d.Containers = []Container{}
	up := map[string]string{}
	if out, err := run("docker", "ps", "--format", "{{.Names}}|{{.Status}}"); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			if name, status, ok := strings.Cut(l, "|"); ok {
				up[name] = status
			}
		}
	}
	if out, err := run("docker", "stats", "--no-stream", "--format", "{{.Name}}|{{.CPUPerc}}|{{.MemUsage}}"); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
			f := strings.Split(l, "|")
			if len(f) != 3 {
				continue
			}
			cpu, _ := strconv.ParseFloat(strings.TrimSuffix(f[1], "%"), 64)
			mem, _, _ := strings.Cut(f[2], " / ")
			d.Containers = append(d.Containers, Container{Name: f[0], Up: up[f[0]], CPUPercent: cpu, MemBytes: parseSize(strings.ReplaceAll(mem, "i", ""))})
		}
	}
	return d
}

// Service is a Homebrew service that starts at login.
type Service struct {
	Name   string `json:"name"`
	Status string `json:"status"` // started, error, none…
}

// BrewServices lists services brew keeps running ("started"); the user
// may not need them on all the time.
func BrewServices(run Runner, look func(string) bool) []Service {
	if !look("brew") {
		return nil
	}
	out, err := run("brew", "services", "list")
	if err != nil {
		return nil
	}
	svcs := []Service{}
	for i, l := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(l)
		if i == 0 || len(f) < 2 { // header: Name Status User File
			continue
		}
		if f[1] == "started" || f[1] == "scheduled" || f[1] == "error" {
			svcs = append(svcs, Service{Name: f[0], Status: f[1]})
		}
	}
	return svcs
}
