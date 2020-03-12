package zsh

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var replacer = strings.NewReplacer(
	`:`, `\:`,
	`"`, `\"`,
	`[`, `\[`,
	`]`, `\]`,
    `'`, `\"`,
)

func snippetFlagCompletion(cmd *cobra.Command, flag *pflag.Flag, action *Action) (snippet string) {
	var suffix string
	if action == nil {
		if flag.NoOptDefVal != "" {
			suffix = "" // no argument required for flag
		} else {
			suffix = " -r" // require a value
		}
	} else {
		suffix = fmt.Sprintf(" -a '(%v)' -r", action.Value)
	}

	//if zshCompFlagCouldBeSpecifiedMoreThenOnce(flag) {
	//	multimark = "*"
	//	multimarkEscaped = "\\*"
	//}

	if flag.Shorthand == "" { // no shorthannd
		snippet = fmt.Sprintf(`complete -c %v -f -n '_state %v' -l %v -d '%v'%v`, cmd.Root().Name(), uidCommand(cmd), flag.Name, replacer.Replace(flag.Usage), suffix)
	} else {
		snippet = fmt.Sprintf(`complete -c %v -f -n '_state %v' -l %v -s %v -d '%v'%v`, cmd.Root().Name(), uidCommand(cmd), flag.Name, flag.Shorthand, replacer.Replace(flag.Usage), suffix)
	}
	return
}

func snippetPositionalCompletion(position int, action Action) string {
	return fmt.Sprintf(`"%v:: :%v"`, position, action.Value)
}

func zshCompFlagCouldBeSpecifiedMoreThenOnce(f *pflag.Flag) bool {
	return strings.Contains(f.Value.Type(), "Slice") ||
		strings.Contains(f.Value.Type(), "Array")
}

func snippetSubcommands(cmd *cobra.Command) string {
	if !cmd.HasSubCommands() {
		return ""
	}
	cmnds := make([]string, 0)
	functions := make([]string, 0)
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			cmnds = append(cmnds, fmt.Sprintf(`        "%v:%v"`, c.Name(), c.Short))
			functions = append(functions, fmt.Sprintf(`    %v)
      %v
      ;;`, c.Name(), uidCommand(c)))

			for _, alias := range c.Aliases {
				cmnds = append(cmnds, fmt.Sprintf(`        "%v:%v"`, alias, c.Short))
				functions = append(functions, fmt.Sprintf(`    %v)
      %v
      ;;`, alias, uidCommand(c)))
			}
		}
	}

	templ := `

  # shellcheck disable=SC2154
  case $state in
    cmnds)
      # shellcheck disable=SC2034
      commands=(
%v
      )
      _describe "command" commands
      ;;
  esac
  
  case "${words[1]}" in
%v
  esac`

	return fmt.Sprintf(templ, strings.Join(cmnds, "\n"), strings.Join(functions, "\n"))
}
