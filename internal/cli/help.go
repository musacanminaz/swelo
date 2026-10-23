package cli

import (
	"strings"

	"github.com/musacanminaz/swelo/internal/i18n"
	"github.com/musacanminaz/swelo/internal/rules"
)

// earlyLang finds the language before the command tree is built, so help
// and flag texts are translated too: --lang on the command line, else the
// config file, else "" (the system language stays).
func earlyLang(args []string) string {
	for i, a := range args {
		if a == "--lang" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--lang="); ok {
			return v
		}
	}
	cfg, _ := rules.ReadConfig(home())
	return cfg.Lang
}

// usageTemplate is cobra's default usage with its headings translated.
func usageTemplate() string {
	t := i18n.T
	return t("help.usage") + `:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [` + t("help.command") + `]{{end}}{{if gt (len .Aliases) 0}}

` + t("help.aliases") + `:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + t("help.examples") + `:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

` + t("help.commands") + `:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + t("help.flags") + `:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

` + t("help.globalflags") + `:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

` + t("help.more") + `{{end}}
`
}
