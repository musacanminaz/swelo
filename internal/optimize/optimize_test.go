package optimize

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimeMachine(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.Mkdir(a, 0o755)
	os.Mkdir(b, 0o755)
	var calls []string
	run := func(argv ...string) (string, error) {
		calls = append(calls, strings.Join(argv, " "))
		switch argv[1] {
		case "destinationinfo":
			return "Name: Backup\n", nil
		case "isexcluded":
			return "[Excluded]  " + a + "\n[Included]  " + b + "\n", nil
		}
		return "", nil
	}
	task := TimeMachine(run, []string{a, b})
	if !task.Ready || len(task.Items) != 1 || task.Items[0] != b {
		t.Fatalf("task = %+v", task)
	}
	task.Apply()
	if last := calls[len(calls)-1]; last != "tmutil addexclusion "+b {
		t.Fatalf("apply ran %q", last)
	}
	off := TimeMachine(func(...string) (string, error) { return "tmutil: No destinations configured.", nil }, []string{a})
	if off.Ready || off.Note == "" {
		t.Fatalf("no destination must skip: %+v", off)
	}
}

func TestGitAndBrew(t *testing.T) {
	root := t.TempDir()
	repo, locked, busy := filepath.Join(root, "repo"), filepath.Join(root, "locked"), filepath.Join(root, "busy")
	for _, r := range []string{repo, locked, busy} {
		os.MkdirAll(r, 0o755)
		if out, err := exec.Command("git", "-C", r, "init", "-q").CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	os.WriteFile(filepath.Join(locked, ".git", "index.lock"), nil, 0o644)
	os.MkdirAll(filepath.Join(root, "app", "node_modules", "dep", ".git"), 0o755)

	repos := FindRepos([]string{root}, map[string]bool{"node_modules": true})
	if len(repos) != 3 {
		t.Fatalf("repos = %v (must not descend into node_modules)", repos)
	}
	task := Git(Real, repos, 1, func(d string) bool { return d == busy })
	if len(task.Items) != 1 || task.Items[0] != repo {
		t.Fatalf("git items = %v (locked and busy repos must be skipped)", task.Items)
	}
	if _, err := task.Apply(); err != nil {
		t.Fatalf("git gc on a fresh repo: %v", err)
	}

	brew := BrewAutoremove(func(...string) (string, error) {
		return "==> Would autoremove 2 unneeded formulae:\nlibfoo\nlibbar\n", nil
	}, func(string) bool { return true })
	if len(brew.Items) != 2 || !brew.Ready {
		t.Fatalf("brew = %+v", brew)
	}
}
