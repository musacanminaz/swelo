package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
)

func TestRulesListAndUnknownRule(t *testing.T) {
	i18n.Set("en")
	rs := []rules.Rule{
		{ID: "node", Group: "build", Kind: "artifact", Names: []string{"node_modules"}, Markers: []string{"package.json"}, Desc: map[string]string{"en": "JS"}},
		{ID: "brew", Group: "tools", Kind: "command", Run: [][]string{{"brew", "cleanup", "-s"}}, Desc: map[string]string{"en": "Brew"}, Disabled: true},
	}
	var out bytes.Buffer
	printRules(&out, rs)
	for _, want := range []string{"build", "node_modules  next to package.json", "brew (disabled in config)", "brew cleanup -s"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
	if j := rulesJSON(rs); len(j) != 2 || j[1].Commands[0] != "brew cleanup -s" || !j[1].Disabled {
		t.Fatalf("%+v", j)
	}
	if knownRule(rs, "tools") != true || knownRule(rs, "nope") {
		t.Fatal("knownRule")
	}
	err := runClean(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{}, t.TempDir(), cleanOpts{only: []string{"nope"}, recentDays: -1}, false)
	if err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("unknown rule accepted: %v", err)
	}
}
