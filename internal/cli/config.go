package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
	"github.com/musacanminaz/swelo/internal/ui"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: i18n.T("cmd.config"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return showConfig(cmd.OutOrStdout(), home())
		},
	}
	// completion: the key, then its values where they are a fixed set
	scalar := func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		switch {
		case len(args) == 0:
			return rules.ScalarKeys, cobra.ShellCompDirectiveNoFileComp
		case len(args) == 1 && args[0] == "lang":
			return []string{"tr", "en"}, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	list := func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		switch {
		case len(args) == 0:
			return rules.ListKeys, cobra.ShellCompDirectiveNoFileComp
		case len(args) == 1 && args[0] == "disable":
			return ruleNames(), cobra.ShellCompDirectiveNoFileComp
		case len(args) == 1:
			return nil, cobra.ShellCompDirectiveFilterDirs
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	c.AddCommand(
		&cobra.Command{
			Use: "set <key> <value>", Short: i18n.T("config.set"), Args: cobra.ExactArgs(2), ValidArgsFunction: scalar,
			RunE: func(cmd *cobra.Command, a []string) error { return setConfig(cmd.OutOrStdout(), home(), a[0], a[1]) },
		},
		&cobra.Command{
			Use: "add <list> <value>", Short: i18n.T("config.add"), Args: cobra.ExactArgs(2), ValidArgsFunction: list,
			RunE: func(cmd *cobra.Command, a []string) error {
				return editList(cmd.OutOrStdout(), home(), a[0], a[1], true)
			},
		},
		&cobra.Command{
			Use: "remove <list> <value>", Short: i18n.T("config.remove"), Args: cobra.ExactArgs(2), ValidArgsFunction: list,
			RunE: func(cmd *cobra.Command, a []string) error {
				return editList(cmd.OutOrStdout(), home(), a[0], a[1], false)
			},
		},
		&cobra.Command{
			Use: "edit", Short: i18n.T("config.edit"),
			RunE: func(cmd *cobra.Command, _ []string) error { return editConfigFile(home()) },
		},
	)
	return c
}

// showConfig prints the file as it is, or the defaults when there is none.
func showConfig(w io.Writer, home string) error {
	path := rules.ConfigPath(home)
	fmt.Fprintf(w, "\n %s %s\n", ui.Bold(i18n.T("config.file")), path)
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(w, " %s\n\n", ui.Dim(i18n.T("config.none")))
	} else {
		lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		for i := range lines {
			lines[i] = ui.Safe(lines[i])
		}
		fmt.Fprintf(w, "\n%s\n", ui.Indent(strings.Join(lines, "\n"), "   "))
	}
	fmt.Fprintf(w, " %s\n", ui.Dim(i18n.T("config.keys")))
	fmt.Fprintf(w, "   %-11s %s\n", "lang", "tr | en")
	fmt.Fprintf(w, "   %-11s %s\n", "recent_days", i18n.T("config.key.recent"))
	fmt.Fprintf(w, "   %-11s %s\n", "top", i18n.T("config.key.top"))
	fmt.Fprintf(w, "   %-11s %s\n", "roots", i18n.T("config.key.roots"))
	fmt.Fprintf(w, "   %-11s %s\n", "exclude", i18n.T("config.key.exclude"))
	fmt.Fprintf(w, "   %-11s %s\n", "protected", i18n.T("config.key.protected"))
	fmt.Fprintf(w, "   %-11s %s\n\n", "disable", i18n.T("config.key.disable"))
	return nil
}

func setConfig(w io.Writer, home, key, value string) error {
	if !slices.Contains(rules.ScalarKeys, key) {
		if slices.Contains(rules.ListKeys, key) {
			return fmt.Errorf(i18n.T("config.islist"), key)
		}
		return fmt.Errorf(i18n.T("config.unknownkey"), key, strings.Join(rules.ScalarKeys, ", "))
	}
	switch key {
	case "lang":
		if value != "tr" && value != "en" {
			return errors.New(i18n.T("config.badlang"))
		}
	default:
		if n, err := strconv.Atoi(value); err != nil || n < 0 {
			return fmt.Errorf(i18n.T("config.badint"), key)
		}
	}
	if err := rules.SetConfig(home, key, value); err != nil {
		return err
	}
	fmt.Fprintf(w, " %s %s: %s\n", ui.Green("✓"), key, ui.Safe(value))
	return nil
}

func editList(w io.Writer, home, key, value string, add bool) error {
	if !slices.Contains(rules.ListKeys, key) {
		return fmt.Errorf(i18n.T("config.unknownlist"), key, strings.Join(rules.ListKeys, ", "))
	}
	if key == "disable" && add {
		rs, _, err := rules.Load(home)
		if err != nil {
			return err
		}
		if !knownRule(rs, value) {
			return fmt.Errorf(i18n.T("clean.unknownrule"), value)
		}
	}
	if key == "roots" && add && !strings.ContainsAny(value, "*?[") {
		if info, err := os.Stat(rules.Expand(value, home)); err != nil || !info.IsDir() {
			return fmt.Errorf(i18n.T("config.nodir"), value)
		}
	}
	var err error
	if add {
		err = rules.AddToConfig(home, key, value)
	} else {
		err = rules.RemoveFromConfig(home, key, value)
	}
	if errors.Is(err, rules.ErrNotInConfig) {
		return fmt.Errorf(i18n.T("config.notin"), value, key)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(w, " %s %s: %s\n", ui.Green("✓"), key, ui.Safe(value))
	return nil
}

// editConfigFile opens config.yaml in $VISUAL / $EDITOR, else the default
// text editor; a missing file is created first so the editor has something.
func editConfigFile(home string) error {
	path := rules.ConfigPath(home)
	if _, err := os.Stat(path); err != nil {
		if err := rules.SetConfig(home, "recent_days", "7"); err != nil {
			return err
		}
	}
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if ed := os.Getenv(env); ed != "" {
			args := append(strings.Fields(ed), path)
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			return cmd.Run()
		}
	}
	return exec.Command("open", "-t", path).Run()
}
