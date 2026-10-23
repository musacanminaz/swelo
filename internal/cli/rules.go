package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func rulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rules",
		Short: i18n.T("cmd.rules"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			rs, _, err := rules.Load(home())
			if err != nil {
				return err
			}
			if jsonFlag {
				return writeJSON(cmd.OutOrStdout(), rulesJSON(rs))
			}
			printRules(cmd.OutOrStdout(), rs)
			return nil
		},
	}
}

// printRules lists every rule by group: risk, id, what it looks at, and
// whether the config turned it off.
func printRules(w io.Writer, rs []rules.Rule) {
	fmt.Fprintf(w, "\n %s\n", ui.Dim(i18n.T("rules.help")))
	group := ""
	for _, r := range rs {
		if r.Group != group {
			group = r.Group
			fmt.Fprintf(w, "\n %s\n", ui.Bold(group))
		}
		mark := ui.Green("●")
		switch r.Risk {
		case rules.Risky:
			mark = ui.Yellow("●")
		case rules.Report:
			mark = ui.Dim("○")
		}
		id := ui.Bold(r.ID)
		if r.Disabled {
			id = ui.Dim(r.ID + " " + i18n.T("rules.disabled"))
		}
		fmt.Fprintf(w, "   %s %s %s\n", mark, ui.Pad(id, 24), r.Description(i18n.Lang()))
		fmt.Fprintf(w, "     %s\n", ui.Dim(ruleScope(r)))
	}
	fmt.Fprintln(w)
}

// ruleScope says what a rule looks at, in the rule's own terms.
func ruleScope(r rules.Rule) string {
	switch r.Kind {
	case "artifact":
		s := strings.Join(r.Names, ", ")
		if len(r.Markers) > 0 {
			s += "  " + fmt.Sprintf(i18n.T("rules.nextto"), strings.Join(r.Markers, ", "))
		}
		return s
	case "path":
		return strings.Join(r.Paths, ", ")
	default:
		var cmds []string
		for _, c := range r.Run {
			cmds = append(cmds, ui.ShellJoin(c))
		}
		return strings.Join(cmds, "; ")
	}
}

type jsonRuleDef struct {
	ID          string   `json:"id"`
	Group       string   `json:"group"`
	Risk        string   `json:"risk"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Names       []string `json:"names,omitempty"`
	Markers     []string `json:"markers,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	Commands    []string `json:"commands,omitempty"`
	Rebuild     string   `json:"rebuild,omitempty"`
	Disabled    bool     `json:"disabled"`
}

func rulesJSON(rs []rules.Rule) []jsonRuleDef {
	out := make([]jsonRuleDef, 0, len(rs))
	for _, r := range rs {
		d := jsonRuleDef{ID: r.ID, Group: r.Group, Risk: r.Risk, Kind: r.Kind, Description: r.Description(i18n.Lang()),
			Names: r.Names, Markers: r.Markers, Paths: r.Paths, Rebuild: r.Rebuild, Disabled: r.Disabled}
		for _, c := range r.Run {
			d.Commands = append(d.Commands, ui.ShellJoin(c))
		}
		out = append(out, d)
	}
	return out
}

// knownRule reports whether only names a rule id or group.
func knownRule(rs []rules.Rule, only string) bool {
	for _, r := range rs {
		if r.ID == only || r.Group == only {
			return true
		}
	}
	return false
}
