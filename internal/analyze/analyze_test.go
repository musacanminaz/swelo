package analyze

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/musacanminaz/swelo/internal/perm"
)

func write(t *testing.T, p string, n int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	home, _ := filepath.EvalSymlinks(t.TempDir())
	write(t, filepath.Join(home, "Desktop", "app", "big.bin"), 3<<20)
	write(t, filepath.Join(home, "Library", "Caches", "Yarn", "x"), 1<<20)
	write(t, filepath.Join(home, "Library", "Caches", "other", "x"), 1<<20)
	write(t, filepath.Join(home, "Library", "Group Containers", "secret", "x"), 1<<20)
	write(t, filepath.Join(home, "notes.txt"), 10)
	os.Link(filepath.Join(home, "Desktop", "app", "big.bin"), filepath.Join(home, "Desktop", "hardlink.bin"))

	cats := Categories(home, []string{filepath.Join(home, "Desktop")})
	r := Scan(Options{Home: home, Categories: cats, Protected: perm.ProtectedDirs(home), BigFile: 2 << 20})

	if r.Sizes["projects"] < 3<<20 || r.Sizes["projects"] > 4<<20 {
		t.Errorf("projects = %d (hard link must be counted once)", r.Sizes["projects"])
	}
	if r.Sizes["pkg-caches"] < 1<<20 || r.Sizes["app-caches"] < 1<<20 {
		t.Errorf("caches = %v", r.Sizes)
	}
	if r.Sizes["library"] != 0 {
		t.Errorf("protected Group Containers was read: library = %d", r.Sizes["library"])
	}
	if len(r.Skipped) != 1 || filepath.Base(r.Skipped[0]) != "Group Containers" {
		t.Errorf("skipped = %v", r.Skipped)
	}
	if len(r.BigFiles) != 1 {
		t.Errorf("big files = %v", r.BigFiles)
	}
	if got := Classify(filepath.Join(home, "Library", "Caches", "Yarn", "v6"), cats); got != "pkg-caches" {
		t.Errorf("Classify = %s", got)
	}

	full := Scan(Options{Home: home, Categories: cats, Protected: perm.ProtectedDirs(home), Full: true})
	if full.Sizes["library"] < 1<<20 {
		t.Errorf("--full must read protected folders: %v", full.Sizes)
	}
}

func TestFileStale(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -200)
	cases := map[File]bool{
		{Path: "/h/movie.mkv", Size: 2e9, Used: old}:                    true,
		{Path: "/h/movie.mkv", Size: 2e9, Used: now.AddDate(0, 0, -10)}: false,
		{Path: "/h/Backups/db.dump", Size: 2e9, Used: old}:              false,
		{Path: "/h/02-DB-Yedekler/x.sql", Size: 2e9, Used: old}:         true, // "02-DB-Yedekler" is not exactly a backup folder name
		{Path: "/h/yedek/x", Size: 2e9, Used: old}:                      false,
		{Path: "/h/x", Size: 2e9}:                                       false, // unknown age
	}
	for f, want := range cases {
		if got := f.Stale(now, 180*24*time.Hour); got != want {
			t.Errorf("%s used %v: stale=%v, want %v", f.Path, f.Used, got, want)
		}
	}
}
