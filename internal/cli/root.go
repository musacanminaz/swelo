// Package cli wires swelo's commands.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/perm"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

// Version is set at build time with -ldflags "-X .../cli.Version=x.y.z".
var Version = "dev"

var (
	langFlag  string
	introFlag bool
)

// Execute runs the root command.
func Execute() error {
	root := &cobra.Command{
		Use:           "swelo",
		Short:         i18n.T("tagline"),
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			i18n.Set(langFlag)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if introFlag {
				os.Remove(introMarker())
			}
			welcome(cmd.OutOrStdout())
			return nil
		},
	}
	root.PersistentFlags().StringVar(&langFlag, "lang", "", "output language: tr | en")
	root.Flags().BoolVar(&introFlag, "intro", false, "show the first-run introduction again before the next scan")
	root.AddCommand(permissionsCmd())
	return root.Execute()
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "/"
	}
	return h
}

// Paths of files swelo writes; shown to the user so nothing is hidden.
func configDir() string { return filepath.Join(home(), ".config", "swelo") }
func stateDir() string  { return filepath.Join(home(), ".local", "state", "swelo") }

func welcome(w io.Writer) {
	fmt.Fprintf(w, "\n %s %s — %s\n\n", ui.Bold("swelo"), Version, i18n.T("tagline"))

	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("what.title")))
	fmt.Fprintln(w, ui.Indent(i18n.T("what.body"), "   "))
	fmt.Fprintln(w)

	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("never.title")))
	for _, k := range []string{"never.1", "never.2", "never.3", "never.4", "never.5"} {
		fmt.Fprintf(w, "   • %s\n", i18n.T(k))
	}
	fmt.Fprintln(w)

	permSummary(w)
	fmt.Fprintf(w, "   %s\n\n", ui.Dim(i18n.T("perm.more")))

	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("cmds.title")))
	cmds := []struct {
		name  string
		ready bool
	}{{"analyze", false}, {"clean", false}, {"optimize", false}, {"uninstall", false}, {"status", false}, {"permissions", true}}
	for _, c := range cmds {
		soon := ""
		if !c.ready {
			soon = " " + ui.Dim(i18n.T("soon"))
		}
		fmt.Fprintf(w, "   %-12s %s%s\n", c.name, i18n.T("cmd."+c.name), soon)
	}
	fmt.Fprintln(w)
}

func permSummary(w io.Writer) {
	fmt.Fprintf(w, " %s\n", ui.Bold(i18n.T("perm.title")))
	fmt.Fprintf(w, "   %s %-22s %s\n", ui.Green("✓"), i18n.T("perm.user"), i18n.T("perm.user.ok"))
	if perm.FullDiskAccess(home()) {
		fmt.Fprintf(w, "   %s %-22s %s\n", ui.Green("✓"), i18n.T("perm.fda"), i18n.T("perm.fda.ok"))
	} else {
		fmt.Fprintf(w, "   %s %-22s %s\n", ui.Yellow("–"), i18n.T("perm.fda"), i18n.T("perm.fda.no"))
	}
	if perm.IsAdmin() {
		fmt.Fprintf(w, "   %s %-22s %s\n", ui.Dim("–"), i18n.T("perm.sudo"), i18n.T("perm.sudo.admin"))
	} else {
		fmt.Fprintf(w, "   %s %-22s %s\n", ui.Yellow("–"), i18n.T("perm.sudo"), i18n.T("perm.sudo.no"))
	}
}

func permissionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "permissions",
		Short: i18n.T("cmd.permissions"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w)
			fmt.Fprintln(w, ui.Indent(i18n.T("permdoc.intro"), " "))
			fmt.Fprintln(w)
			permSummary(w)
			for _, s := range []struct{ title, body string }{
				{i18n.T("perm.user"), i18n.T("permdoc.user")},
				{i18n.T("perm.fda"), i18n.T("permdoc.fda")},
				{i18n.T("perm.sudo"), i18n.T("permdoc.sudo")},
				{"Network", i18n.T("permdoc.net")},
			} {
				fmt.Fprintf(w, "\n %s\n%s\n", ui.Bold(s.title), ui.Indent(s.body, "   "))
			}
			fmt.Fprintf(w, "\n %s\n   %s\n   %s\n\n", ui.Bold(i18n.T("permdoc.files")),
				filepath.Join(configDir(), "config.yaml"), filepath.Join(stateDir(), "history.jsonl"))
			return nil
		},
	}
}
