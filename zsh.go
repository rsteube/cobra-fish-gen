package zsh

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type Completions struct {
	actions map[string]Action
}

func (c Completions) invokeCallback(uid string, args []string) Action {
	if action, ok := c.actions[uid]; ok {
		if action.Callback != nil {
			return action.Callback(args)
		}
	}
	return Action{Value: ""} // no message on fish since callback is determinaed by commandline
	//return ActionMessage(fmt.Sprintf("callback %v unknown", uid))
}

func (c Completions) Generate(cmd *cobra.Command) string {
	result := fmt.Sprintf(`function _state
  set -lx CURRENT (commandline -cp)
  if [ "$LINE" != "$CURRENT" ]
    set -gx LINE (commandline -cp)
    set -gx STATE (commandline -cp | xargs %v _fish_completion state)
  end

  [ "$STATE" = "$argv" ]
end

function _callback
  set -lx CALLBACK (commandline -cp | sed "s/ \$/ _/" | xargs %v _fish_completion $argv )
  eval "$CALLBACK"
end

complete -c %v -f
`, cmd.Name(), cmd.Name(), cmd.Name())
	result += c.GenerateFunctions(cmd)

	return result
}

func (c Completions) GenerateFunctions(cmd *cobra.Command) string {
  // TODO ensure state is only called oncy per LINE
	function_pattern := `
%v
`

	flags := make([]string, 0)
	for _, flag := range zshCompExtractFlag(cmd) {
		if flagAlreadySet(cmd, flag) {
			continue
		}

		var s string
		if action, ok := c.actions[uidFlag(cmd, flag)]; ok {
			s = snippetFlagCompletion(cmd, flag, &action)
		} else {
			s = snippetFlagCompletion(cmd, flag, nil)
		}

		flags = append(flags, s)
	}

	positionals := make([]string, 0)
	if cmd.HasSubCommands() {
		positionals = []string{}
		for _, subcmd := range cmd.Commands() {
          positionals = append(positionals, fmt.Sprintf(`complete -c %v -f -n '_state %v ' -a %v -d '%v'`, cmd.Root().Name(), uidCommand(cmd), subcmd.Name(), subcmd.Short))
			// TODO repeat for aliases
			// TODO filter hidden
		}
	} else {
		if len(positionals) == 0 {
			if cmd.ValidArgs != nil {
				//positionals = []string{"    " + snippetPositionalCompletion(1, ActionValues(cmd.ValidArgs...))}
			}
			positionals = append(positionals, fmt.Sprintf(`complete -c %v -f -n '_state %v' -a '(_callback _)'`, cmd.Root().Name(), uidCommand(cmd)))
		}
	}

	arguments := append(flags, positionals...)

	result := make([]string, 0)
	result = append(result, fmt.Sprintf(function_pattern,  strings.Join(arguments, "\n")))
	for _, subcmd := range cmd.Commands() {
		if !subcmd.Hidden {
			result = append(result, c.GenerateFunctions(subcmd))
		}
	}
	return strings.Join(result, "\n")
}

func flagAlreadySet(cmd *cobra.Command, flag *pflag.Flag) bool {
	if cmd.LocalFlags().Lookup(flag.Name) != nil {
		return false
	}
	// TODO since it is an inherited flag check for parent command that is not hidden
	return true
}

func zshCompExtractFlag(c *cobra.Command) []*pflag.Flag {
	var flags []*pflag.Flag
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flags = append(flags, f)
		}
	})
	c.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			flags = append(flags, f)
		}
	})
	return flags
}

type ZshCompletion struct {
	cmd *cobra.Command
}

func Gen(cmd *cobra.Command) *ZshCompletion {
	addCompletionCommand(cmd)
	return &ZshCompletion{
		cmd: cmd,
	}
}

func (zsh ZshCompletion) PositionalCompletion(action ...Action) {
	for index, a := range action {
		completions.actions[uidPositional(zsh.cmd, index+1)] = a.finalize(uidPositional(zsh.cmd, index+1))
	}
}

func (zsh ZshCompletion) FlagCompletion(actions ActionMap) {
	for name, action := range actions {
		flag := zsh.cmd.Flag(name) // TODO only allowed for local flags
		completions.actions[uidFlag(zsh.cmd, flag)] = action.finalize(uidFlag(zsh.cmd, flag))
	}
}

var completions = Completions{
	actions: make(map[string]Action),
}

func addCompletionCommand(cmd *cobra.Command) {
	for _, c := range cmd.Root().Commands() {
		if c.Name() == "_zsh_completion" {
			return
		}
	}
	cmd.Root().AddCommand(&cobra.Command{
		Use:    "_zsh_completion",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) <= 0 {
				fmt.Println(completions.Generate(cmd.Root()))
			} else {
				callback := args[0]
				origArg := []string{}
				if len(os.Args) > 3 {
					origArg = os.Args[4:]
				}
				_, targetArgs := traverse(cmd, origArg)
				fmt.Println(completions.invokeCallback(callback, targetArgs).Value)
			}
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		DisableFlagParsing: true,
	})

	cmd.Root().AddCommand(&cobra.Command{
		Use:    "_fish_completion",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) <= 0 {
				fmt.Println(completions.Generate(cmd.Root()))
			} else {
				callback := args[0]
				origArg := []string{}
				if len(os.Args) > 4 {
					origArg = os.Args[4:]
				}
				targetCmd, targetArgs := traverse(cmd, origArg)
				if callback == "_" {
					if len(targetArgs) == 0 {
						callback = uidPositional(targetCmd, 1)
					} else {
						lastArg := targetArgs[len(targetArgs)-1]
						if strings.HasSuffix(lastArg, " ") {
							callback = uidPositional(targetCmd, len(targetArgs)+1)
						} else {
							callback = uidPositional(targetCmd, len(targetArgs))
						}
					}
				} else if callback == "state" {
					 fmt.Println(uidCommand(targetCmd))
                     os.Exit(0) // TODO
				}
				fmt.Println(completions.invokeCallback(callback, targetArgs).Value)
			}
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		DisableFlagParsing: true,
	})
}

func traverse(cmd *cobra.Command, args []string) (*cobra.Command, []string) {
	// ignore flag parse errors (like a missing argument for the flag currently being completed)
	targetCmd, targetArgs, _ := cmd.Root().Traverse(args)
	targetCmd.ParseFlags(targetArgs)
	return targetCmd, targetCmd.Flags().Args() // TODO check length
}
