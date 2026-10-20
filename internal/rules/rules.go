// Package rules loads swelo's cleanup rules and user configuration.
package rules

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultYAML []byte

// Risk levels.
const (
	Safe   = "safe"
	Risky  = "risky"
	Report = "report"
)

// Rule is one cleanup rule; see default.yaml for the meaning of each field.
type Rule struct {
	ID            string            `yaml:"id"`
	Group         string            `yaml:"group"`
	Risk          string            `yaml:"risk"`
	Kind          string            `yaml:"kind"`
	Desc          map[string]string `yaml:"desc"`
	Rebuild       string            `yaml:"rebuild"`
	Running       []string          `yaml:"running"`
	Names         []string          `yaml:"names"`
	Markers       []string          `yaml:"markers"`
	Paths         []string          `yaml:"paths"`
	Children      bool              `yaml:"children"`
	NameGlob      string            `yaml:"name_glob"`
	OlderThanDays int               `yaml:"older_than_days"`
	SkipIfChild   string            `yaml:"skip_if_child"`
	KeepLatest    int               `yaml:"keep_latest"`
	Requires      string            `yaml:"requires"`
	Check         []string          `yaml:"check"`
	Show          []string          `yaml:"show"`
	SizeCmd       []string          `yaml:"size_cmd"`      // prints a path to measure
	SizeTextCmd   []string          `yaml:"size_text_cmd"` // prints a size like "1.2GB"
	Run           [][]string        `yaml:"run"`
	Disabled      bool              `yaml:"disabled"`
}

// Description returns the rule description in lang, falling back to English.
func (r Rule) Description(lang string) string {
	if d := r.Desc[lang]; d != "" {
		return d
	}
	return r.Desc["en"]
}

// Config is ~/.config/swelo/config.yaml.
type Config struct {
	Roots      []string `yaml:"roots"`       // project folders to scan
	Exclude    []string `yaml:"exclude"`     // globs never touched
	Protected  []string `yaml:"protected"`   // extra protected folders
	RecentDays *int     `yaml:"recent_days"` // skip projects changed within N days (default 7)
	Disable    []string `yaml:"disable"`     // rule ids or groups to turn off
	Top        int      `yaml:"top"`         // items listed per rule (default 10)
}

// DefaultRoots are scanned for projects when the config names none.
var DefaultRoots = []string{"~/Desktop", "~/Projects", "~/projects", "~/Developer", "~/code", "~/dev", "~/src", "~/work", "~/repos", "~/git", "~/GitHub"}

// DefaultProtected are never deleted, whatever a rule says.
var DefaultProtected = []string{
	"~/Documents", "~/Pictures", "~/Movies", "~/Music", "~/.ssh", "~/.gnupg", "~/.aws", "~/.kube",
	"~/Library/Keychains", "~/Library/Mobile Documents", "~/Library/Mail", "~/Library/Messages",
	"~/Library/Containers/com.docker.docker", "~/Library/Application Support/AddressBook",
}

// Load reads the default rules, then the user's rules file (same id overrides,
// new ids are appended) and config. Missing user files are fine.
func Load(home string) ([]Rule, Config, error) {
	var rs []Rule
	if err := yaml.Unmarshal(defaultYAML, &rs); err != nil {
		return nil, Config{}, fmt.Errorf("default rules: %w", err)
	}
	dir := filepath.Join(home, ".config", "swelo")
	if b, err := os.ReadFile(filepath.Join(dir, "rules.yaml")); err == nil {
		var user []Rule
		if err := yaml.Unmarshal(b, &user); err != nil {
			return nil, Config{}, fmt.Errorf("rules.yaml: %w", err)
		}
		rs = merge(rs, user)
	}
	var cfg Config
	if b, err := os.ReadFile(filepath.Join(dir, "config.yaml")); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return nil, Config{}, fmt.Errorf("config.yaml: %w", err)
		}
	}
	if cfg.RecentDays == nil {
		d := 7
		cfg.RecentDays = &d
	}
	if cfg.Top == 0 {
		cfg.Top = 10
	}
	if len(cfg.Roots) == 0 {
		cfg.Roots = DefaultRoots
	}
	cfg.Protected = append(append([]string{}, DefaultProtected...), cfg.Protected...)
	for i := range rs {
		if rs[i].Risk == "" {
			rs[i].Risk = Safe
		}
		for _, d := range cfg.Disable {
			if d == rs[i].ID || d == rs[i].Group {
				rs[i].Disabled = true
			}
		}
		if err := validate(rs[i]); err != nil {
			return nil, Config{}, err
		}
	}
	return rs, cfg, nil
}

func merge(base, user []Rule) []Rule {
	idx := map[string]int{}
	for i, r := range base {
		idx[r.ID] = i
	}
	for _, u := range user {
		if i, ok := idx[u.ID]; ok {
			base[i] = u
		} else {
			base = append(base, u)
		}
	}
	return base
}

func validate(r Rule) error {
	bad := func(msg string) error { return fmt.Errorf("rule %q: %s", r.ID, msg) }
	if r.ID == "" {
		return fmt.Errorf("rule without id")
	}
	switch r.Risk {
	case Safe, Risky, Report:
	default:
		return bad("risk must be safe, risky or report")
	}
	switch r.Kind {
	case "artifact":
		if len(r.Names) == 0 {
			return bad("artifact rule needs names")
		}
	case "path":
		if len(r.Paths) == 0 {
			return bad("path rule needs paths")
		}
	case "command":
		if len(r.Run) == 0 {
			return bad("command rule needs run")
		}
	default:
		return bad("kind must be artifact, path or command")
	}
	return nil
}

// Expand replaces a leading ~ and $UID in p.
func Expand(p, home string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[1:])
	}
	return strings.ReplaceAll(p, "$UID", fmt.Sprint(os.Getuid()))
}
