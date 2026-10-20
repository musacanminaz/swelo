// Package uninstall lists installed developer toolchains and removes them with
// each tool's own uninstaller (never a plain rm, except root-owned JDKs via sudo).
package uninstall

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/musacanminaz/swelo/internal/clean"
	"github.com/musacanminaz/swelo/internal/perm"
)

// Package is one removable item.
type Package struct {
	Provider  string // e.g. "brew", "node"
	ID        string // unique: provider:name
	Name      string // what the user sees
	Size      int64  // bytes on disk (0 = unknown)
	Installed time.Time
	LastUsed  time.Time // only when the tool records it (simulator runtimes)
	UsedBy    []string  // projects that pin this version
	Note      string    // e.g. "default version", "used by a container"
	Command   []string  // the exact uninstall command
	Sudo      bool
}

// Env is what providers may touch; tests replace Run and Look.
type Env struct {
	Home  string
	Roots []string // project roots, for "used by" checks
	Run   func(argv ...string) (string, error)
	Look  func(name string) bool
	// System locations, replaceable in tests.
	JDKDir, AndroidSDK string
	AppDirs            []string
	FDA                bool                  // Full Disk Access: app Containers may be read
	Running            func(app string) bool // is this .app running
	// Progress, if set, is told when a provider starts (n < 0) and how many
	// packages it found when it ends.
	Progress func(provider string, n int)
	pins     map[string][]pin
}

type pin struct{ project, version string }

// NewEnv returns an Env that runs real commands.
func NewEnv(home string, roots []string) *Env {
	return &Env{
		Home:  home,
		Roots: roots,
		Run: func(argv ...string) (string, error) {
			out, err := exec.Command(argv[0], argv[1:]...).Output()
			return string(out), err
		},
		Look:    func(name string) bool { _, err := exec.LookPath(name); return err == nil },
		JDKDir:  "/Library/Java/JavaVirtualMachines",
		AppDirs: []string{"/Applications", filepath.Join(home, "Applications")},
		FDA:     perm.FullDiskAccess(home),
		Running: runningApps(),
		AndroidSDK: func() string {
			if s := os.Getenv("ANDROID_HOME"); s != "" {
				return s
			}
			return filepath.Join(home, "Library", "Android", "sdk")
		}(),
	}
}

// Provider lists one kind of package.
type Provider struct {
	ID   string
	List func(*Env) []Package
}

// Providers in display order.
var Providers = []Provider{
	{"app", apps},
	{"simruntime", simRuntimes},
	{"android", androidSDK},
	{"brew", brew},
	{"node", nodeVersions},
	{"python", pythonVersions},
	{"rust", rustToolchains},
	{"jdk", jdks},
	{"npm-global", npmGlobals},
	{"docker-image", dockerImages},
	{"vscode", vscodeExtensions},
	{"ollama", ollamaModels},
}

// List runs every provider whose tool is present. A cask that another provider
// already removes through brew (a JDK, an app) is shown once.
func List(e *Env, only []string) []Package {
	var all []Package
	for _, p := range Providers {
		if len(only) > 0 && !contains(only, p.ID) {
			continue
		}
		if e.Progress != nil {
			e.Progress(p.ID, -1)
		}
		found := p.List(e)
		if e.Progress != nil {
			e.Progress(p.ID, len(found))
		}
		all = append(all, found...)
	}
	viaJDK := map[string]bool{}
	for _, p := range all {
		if p.Provider != "brew" && len(p.Command) == 4 && p.Command[0] == "brew" && p.Command[2] == "--cask" {
			viaJDK["cask:"+p.Command[3]] = true
		}
	}
	out := all[:0]
	for _, p := range all {
		if !viaJDK[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func mtime(p string) time.Time {
	if info, err := os.Stat(p); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func subdirs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

// ---- version pins in projects (.nvmrc, .python-version, .tool-versions) ----

func (e *Env) pinned(tool, version string) []string {
	if e.pins == nil {
		e.pins = scanPins(e.Roots)
	}
	var out []string
	for _, p := range e.pins[tool] {
		v := strings.TrimPrefix(p.version, "v")
		ver := strings.TrimPrefix(version, "v")
		if v != "" && (ver == v || strings.HasPrefix(ver, v+".")) {
			out = append(out, p.project)
		}
	}
	return out
}

var skipDirs = map[string]bool{"node_modules": true, ".git": true, "build": true, "dist": true, ".venv": true, "Pods": true, "target": true, ".next": true}

func scanPins(roots []string) map[string][]pin {
	pins := map[string][]pin{}
	add := func(tool, project, v string) {
		if v = strings.TrimSpace(v); v != "" {
			pins[tool] = append(pins[tool], pin{project, v})
		}
	}
	for _, root := range roots {
		base := strings.Count(root, "/")
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skipDirs[d.Name()] || strings.Count(p, "/")-base > 5 {
					return filepath.SkipDir
				}
				return nil
			}
			dir := filepath.Dir(p)
			switch d.Name() {
			case ".nvmrc", ".node-version":
				b, _ := os.ReadFile(p)
				add("node", dir, string(b))
			case ".python-version":
				b, _ := os.ReadFile(p)
				add("python", dir, string(b))
			case ".tool-versions":
				f, _ := os.Open(p)
				if f == nil {
					return nil
				}
				sc := bufio.NewScanner(f)
				for sc.Scan() {
					fs := strings.Fields(sc.Text())
					if len(fs) >= 2 {
						tool := map[string]string{"nodejs": "node", "python": "python"}[fs[0]]
						add(tool, dir, fs[1])
					}
				}
				f.Close()
			}
			return nil
		})
	}
	return pins
}

// ---- providers ----

func brew(e *Env) []Package {
	if !e.Look("brew") {
		return nil
	}
	cellar, _ := e.Run("brew", "--cellar")
	caskroom, _ := e.Run("brew", "--caskroom")
	cellar, caskroom = strings.TrimSpace(cellar), strings.TrimSpace(caskroom)
	var out []Package
	leaves, _ := e.Run("brew", "leaves", "--installed-on-request")
	for _, name := range strings.Fields(leaves) {
		dir := filepath.Join(cellar, filepath.Base(name))
		out = append(out, Package{Provider: "brew", ID: "brew:" + name, Name: name, Size: clean.DirSize(dir),
			Installed: mtime(dir), Command: []string{"brew", "uninstall", name}})
	}
	casks, _ := e.Run("brew", "list", "--cask")
	for _, name := range strings.Fields(casks) {
		dir := filepath.Join(caskroom, name)
		out = append(out, Package{Provider: "brew", ID: "cask:" + name, Name: name + " (cask)", Size: clean.DirSize(dir),
			Installed: mtime(dir), Note: "app size not included", Command: []string{"brew", "uninstall", "--cask", name}})
	}
	return out
}

func nodeVersions(e *Env) []Package {
	dir := filepath.Join(e.Home, ".nvm", "versions", "node")
	def, _ := os.ReadFile(filepath.Join(e.Home, ".nvm", "alias", "default"))
	defv := strings.TrimPrefix(strings.TrimSpace(string(def)), "v")
	var out []Package
	for _, v := range subdirs(dir) {
		p := Package{Provider: "node", ID: "node:" + v, Name: "Node " + v, Size: clean.DirSize(filepath.Join(dir, v)),
			Installed: mtime(filepath.Join(dir, v)), UsedBy: e.pinned("node", v),
			Command: []string{"bash", "-c", `export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"; . "$NVM_DIR/nvm.sh" && nvm uninstall "$1"`, "nvm", v}}
		if defv != "" && (strings.TrimPrefix(v, "v") == defv || strings.HasPrefix(strings.TrimPrefix(v, "v"), defv+".")) {
			p.Note = "nvm default"
		}
		out = append(out, p)
	}
	return out
}

func pythonVersions(e *Env) []Package {
	dir := filepath.Join(e.Home, ".pyenv", "versions")
	var out []Package
	for _, v := range subdirs(dir) {
		out = append(out, Package{Provider: "python", ID: "python:" + v, Name: "Python " + v + " (pyenv)",
			Size: clean.DirSize(filepath.Join(dir, v)), Installed: mtime(filepath.Join(dir, v)),
			UsedBy: e.pinned("python", v), Command: []string{"pyenv", "uninstall", "-f", v}})
	}
	return out
}

func rustToolchains(e *Env) []Package {
	dir := filepath.Join(e.Home, ".rustup", "toolchains")
	def := ""
	if e.Look("rustup") {
		def, _ = e.Run("rustup", "default")
	}
	var out []Package
	for _, t := range subdirs(dir) {
		p := Package{Provider: "rust", ID: "rust:" + t, Name: "Rust " + t, Size: clean.DirSize(filepath.Join(dir, t)),
			Installed: mtime(filepath.Join(dir, t)), Command: []string{"rustup", "toolchain", "uninstall", t}}
		if strings.HasPrefix(strings.TrimSpace(def), t) {
			p.Note = "rustup default"
		}
		out = append(out, p)
	}
	return out
}

func simRuntimes(e *Env) []Package {
	if !e.Look("xcrun") {
		return nil
	}
	js, err := e.Run("xcrun", "simctl", "runtime", "list", "-j")
	if err != nil {
		return nil
	}
	var m map[string]struct {
		Identifier, Version, PlatformIdentifier, State string
		SizeBytes                                      int64
		LastUsedAt                                     time.Time
		Deletable                                      bool
	}
	if json.Unmarshal([]byte(js), &m) != nil {
		return nil
	}
	var out []Package
	for _, r := range m {
		plat := strings.TrimSuffix(strings.TrimPrefix(r.PlatformIdentifier, "com.apple.platform."), "simulator")
		if name, ok := map[string]string{"iphone": "iOS", "appletv": "tvOS", "watch": "watchOS", "xr": "visionOS"}[plat]; ok {
			plat = name
		}
		p := Package{Provider: "simruntime", ID: "simruntime:" + r.Identifier, Name: plat + " " + r.Version + " simulator runtime",
			Size: r.SizeBytes, LastUsed: r.LastUsedAt, Command: []string{"xcrun", "simctl", "runtime", "delete", r.Identifier}}
		if !r.Deletable {
			p.Note = "not deletable (bundled with Xcode)"
			p.Command = nil
		}
		out = append(out, p)
	}
	return out
}

func androidSDK(e *Env) []Package {
	sdk := e.AndroidSDK
	if sdk == "" {
		return nil
	}
	mgr := filepath.Join(sdk, "cmdline-tools", "latest", "bin", "sdkmanager")
	if _, err := os.Stat(mgr); err != nil {
		return nil
	}
	var out []Package
	add := func(id, dir string) {
		out = append(out, Package{Provider: "android", ID: "android:" + id, Name: id, Size: clean.DirSize(dir),
			Installed: mtime(dir), Command: []string{mgr, "--uninstall", id}})
	}
	for _, kind := range []string{"build-tools", "platforms", "ndk"} {
		for _, v := range subdirs(filepath.Join(sdk, kind)) {
			add(kind+";"+v, filepath.Join(sdk, kind, v))
		}
	}
	for _, api := range subdirs(filepath.Join(sdk, "system-images")) {
		for _, tag := range subdirs(filepath.Join(sdk, "system-images", api)) {
			for _, abi := range subdirs(filepath.Join(sdk, "system-images", api, tag)) {
				add("system-images;"+api+";"+tag+";"+abi, filepath.Join(sdk, "system-images", api, tag, abi))
			}
		}
	}
	if _, err := os.Stat(filepath.Join(sdk, "emulator")); err == nil {
		add("emulator", filepath.Join(sdk, "emulator"))
	}
	return out
}

func jdks(e *Env) []Package {
	dir := e.JDKDir
	if dir == "" {
		return nil
	}
	casks := map[string]bool{}
	if e.Look("brew") {
		list, _ := e.Run("brew", "list", "--cask")
		for _, c := range strings.Fields(list) {
			casks[c] = true
		}
	}
	var out []Package
	for _, j := range subdirs(dir) {
		p := filepath.Join(dir, j)
		pkg := Package{Provider: "jdk", ID: "jdk:" + j, Name: "JDK " + strings.TrimSuffix(j, ".jdk"),
			Size: clean.DirSize(p), Installed: mtime(p), Sudo: true, Command: []string{"rm", "-rf", p},
			Note: "root-owned; if installed with Homebrew, uninstall the cask instead"}
		if c := jdkCask(j, casks); c != "" {
			// removing a cask-installed JDK by hand would leave Homebrew out of sync
			pkg.Sudo, pkg.Command = false, []string{"brew", "uninstall", "--cask", c}
			pkg.Note = "installed by Homebrew cask " + c + ""
		}
		out = append(out, pkg)
	}
	return out
}

// jdkCask maps "zulu-17.jdk" / "temurin-21.jdk" to an installed cask such as
// "zulu@17", "temurin@21", "zulu17" or "temurin".
func jdkCask(dir string, casks map[string]bool) string {
	name := strings.TrimSuffix(dir, ".jdk")
	vendor, ver, _ := strings.Cut(name, "-")
	if i := strings.IndexAny(ver, ".+"); i > 0 {
		ver = ver[:i]
	}
	for _, c := range []string{vendor + "@" + ver, vendor + ver, vendor} {
		if casks[c] {
			return c
		}
	}
	return ""
}

func npmGlobals(e *Env) []Package {
	if !e.Look("npm") {
		return nil
	}
	js, err := e.Run("npm", "ls", "-g", "--depth=0", "--json")
	if err != nil && js == "" {
		return nil
	}
	var m struct {
		Dependencies map[string]struct{ Version string }
	}
	json.Unmarshal([]byte(js), &m)
	root, _ := e.Run("npm", "root", "-g")
	root = strings.TrimSpace(root)
	var out []Package
	for name, d := range m.Dependencies {
		if name == "npm" || name == "corepack" {
			continue // ships with Node
		}
		dir := filepath.Join(root, name)
		out = append(out, Package{Provider: "npm-global", ID: "npm:" + name, Name: name + "@" + d.Version,
			Size: clean.DirSize(dir), Installed: mtime(dir), Command: []string{"npm", "rm", "-g", name}})
	}
	return out
}

func dockerImages(e *Env) []Package {
	if !e.Look("docker") {
		return nil
	}
	if _, err := e.Run("docker", "info"); err != nil {
		return nil
	}
	lines, _ := e.Run("docker", "images", "--format", "{{json .}}")
	var out []Package
	for _, l := range strings.Split(strings.TrimSpace(lines), "\n") {
		var im struct{ Repository, Tag, ID, Size, Containers string }
		if json.Unmarshal([]byte(l), &im) != nil {
			continue
		}
		p := Package{Provider: "docker-image", ID: "docker:" + im.ID, Name: im.Repository + ":" + im.Tag,
			Size: clean.ParseSize(im.Size), Command: []string{"docker", "rmi", im.ID}}
		if im.Containers != "" && im.Containers != "0" && im.Containers != "N/A" {
			p.Note = "used by a container (stopped or running)"
		}
		out = append(out, p)
	}
	return out
}

var extVersion = regexp.MustCompile(`-\d+\.\d+.*$`)

func vscodeExtensions(e *Env) []Package {
	var out []Package
	for _, ed := range []struct{ dir, cli, label string }{{".vscode", "code", "VS Code"}, {".cursor", "cursor", "Cursor"}} {
		dir := filepath.Join(e.Home, ed.dir, "extensions")
		if !e.Look(ed.cli) {
			continue
		}
		for _, x := range subdirs(dir) {
			id := extVersion.ReplaceAllString(x, "")
			p := filepath.Join(dir, x)
			out = append(out, Package{Provider: "vscode", ID: ed.cli + ":" + x, Name: ed.label + " " + id,
				Size: clean.DirSize(p), Installed: mtime(p), Command: []string{ed.cli, "--uninstall-extension", id}})
		}
	}
	return out
}

func ollamaModels(e *Env) []Package {
	if !e.Look("ollama") {
		return nil
	}
	list, err := e.Run("ollama", "list")
	if err != nil {
		return nil
	}
	var out []Package
	for i, l := range strings.Split(strings.TrimSpace(list), "\n") {
		f := strings.Fields(l)
		if i == 0 || len(f) < 4 {
			continue // header
		}
		out = append(out, Package{Provider: "ollama", ID: "ollama:" + f[0], Name: f[0],
			Size: clean.ParseSize(f[2] + f[3]), Command: []string{"ollama", "rm", f[0]}})
	}
	return out
}

// SortBySize orders packages largest first within each provider, providers in Providers order.
func SortBySize(ps []Package) {
	order := map[string]int{}
	for i, p := range Providers {
		order[p.ID] = i
	}
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Provider != ps[j].Provider {
			return order[ps[i].Provider] < order[ps[j].Provider]
		}
		return ps[i].Size > ps[j].Size
	})
}

// runningApps snapshots running processes once and reports whether an .app's
// executable is among them.
func runningApps() func(string) bool {
	out, _ := exec.Command("ps", "-axo", "comm=").Output()
	procs := string(out)
	return func(app string) bool { return strings.Contains(procs, app+"/Contents/MacOS/") }
}
