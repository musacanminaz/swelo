package rules

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config keys swelo config can change: scalars and lists.
var (
	ScalarKeys = []string{"lang", "recent_days", "top"}
	ListKeys   = []string{"roots", "exclude", "protected", "disable"}
)

// ConfigPath is ~/.config/swelo/config.yaml.
func ConfigPath(home string) string { return filepath.Join(home, ".config", "swelo", "config.yaml") }

// ReadConfig returns the user's config file as written (no defaults filled
// in); a missing file is an empty Config.
func ReadConfig(home string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(ConfigPath(home))
	if err != nil {
		return cfg, nil
	}
	return cfg, yaml.Unmarshal(b, &cfg)
}

// SetConfig sets a scalar key in config.yaml, keeping everything else in the
// file (comments included) as it is.
func SetConfig(home, key, value string) error {
	return editConfig(home, func(m *yaml.Node) error {
		mapSet(m, key, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
		return nil
	})
}

// AddToConfig appends value to a list key unless it is already there.
func AddToConfig(home, key, value string) error {
	return editConfig(home, func(m *yaml.Node) error {
		seq := mapGet(m, key)
		if seq == nil || seq.Kind != yaml.SequenceNode {
			seq = &yaml.Node{Kind: yaml.SequenceNode}
			mapSet(m, key, seq)
		}
		for _, v := range seq.Content {
			if v.Value == value {
				return nil
			}
		}
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: value})
		return nil
	})
}

// RemoveFromConfig drops value from a list key; ErrNotInConfig if absent.
func RemoveFromConfig(home, key, value string) error {
	return editConfig(home, func(m *yaml.Node) error {
		seq := mapGet(m, key)
		if seq == nil || seq.Kind != yaml.SequenceNode {
			return ErrNotInConfig
		}
		for i, v := range seq.Content {
			if v.Value == value {
				seq.Content = append(seq.Content[:i], seq.Content[i+1:]...)
				return nil
			}
		}
		return ErrNotInConfig
	})
}

// ErrNotInConfig: the value to remove is not in the list.
var ErrNotInConfig = errors.New("not in config")

func editConfig(home string, edit func(m *yaml.Node) error) error {
	path := ConfigPath(home)
	var doc yaml.Node
	if b, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(b)) > 0 {
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return err
		}
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	m := doc.Content[0]
	if m.Kind != yaml.MappingNode {
		return errors.New("config.yaml is not a map")
	}
	if err := edit(m); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	enc.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func mapGet(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mapSet(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, v)
}
