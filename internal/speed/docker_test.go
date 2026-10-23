package speed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockerInfoAndServices(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Group Containers", "group.com.docker")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "settings-store.json"), []byte(`{"MemoryMiB":8092,"Cpus":8,"UseResourceSaver":true}`), 0o644)
	run := func(argv ...string) (string, error) {
		switch argv[1] {
		case "info":
			return "", nil
		case "ps":
			return "pg|Up 4 days (healthy)\nmailpit|Up 4 days\n", nil
		case "stats":
			return "pg|2.07%|151.5MiB / 7.653GiB\nmailpit|0.00%|48.84MiB / 7.653GiB\n", nil
		case "services":
			return "Name        Status User File\ncloudflared none\npostgresql@16 started me ~/Library/LaunchAgents/x.plist\nredis error 256 me\n", nil
		}
		return "", nil
	}
	have := func(string) bool { return true }
	d := DockerInfo(run, have, home, true)
	if !d.Installed || !d.Running || !d.SettingsRead || d.MemoryBytes != 8092<<20 || d.CPUs != 8 || !d.ResourceSaver {
		t.Fatalf("%+v", d)
	}
	if len(d.Containers) != 2 || d.Containers[0].Name != "pg" || d.Containers[0].Idle() || !d.Containers[1].Idle() ||
		d.Containers[1].Up != "Up 4 days" || d.Containers[0].MemBytes != parseSize("151.5M") {
		t.Fatalf("%+v", d.Containers)
	}
	if d := DockerInfo(run, have, home, false); d.SettingsRead || d.MemoryBytes != 0 {
		t.Fatal("settings read without Full Disk Access")
	}
	svcs := BrewServices(run, have)
	if len(svcs) != 2 || svcs[0].Name != "postgresql@16" || svcs[0].Status != "started" || svcs[1].Status != "error" {
		t.Fatalf("%+v", svcs)
	}
	if BrewServices(run, func(string) bool { return false }) != nil || len(BrewServices(func(...string) (string, error) { return "Name Status\n", nil }, have)) != 0 {
		t.Fatal("brew missing")
	}
}
