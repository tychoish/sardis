package operations

import (
	"fmt"
	"strings"

	"github.com/cheynewallace/tabby"
	"github.com/tychoish/fun"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

type FlagOptions[T any] struct {
	Name        string
	Usage       string
	EnvVar      string
	FilePath    string
	Required    bool
	Hidden      bool
	TakesFile   bool
	Default     T
	Destination *T
}

func MakeFlag[T any](opts FlagOptions[T]) (out cli.Flag) {
	switch any(fun.ZeroOf[T]()).(type) {
	case string:
		out = cli.StringFlag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(string),
			Destination: any(opts.Destination).(*string),
		}
	case []string:
		o := cli.StringSliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.StringSlice)
			o.Value = vd
		} else {
			vd := cli.StringSlice(any(opts.Default).([]string))
			o.Value = &vd
		}

		out = o
	case []int:
		o := cli.IntSliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.IntSlice)
			o.Value = vd
		} else {
			vd := cli.IntSlice(any(opts.Default).([]int))
			o.Value = &vd
		}

		out = o
	case int:
		out = cli.IntFlag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(int),
			Destination: any(opts.Destination).(*int),
		}
	case bool:
		if any(opts.Default).(bool) {
			out = cli.BoolTFlag{
				Name:        opts.Name,
				Usage:       opts.Usage,
				EnvVar:      opts.EnvVar,
				FilePath:    opts.FilePath,
				Required:    opts.Required,
				Hidden:      opts.Hidden,
				Destination: any(opts.Destination).(*bool),
			}
		} else {
			out = cli.BoolFlag{
				Name:        opts.Name,
				Usage:       opts.Usage,
				EnvVar:      opts.EnvVar,
				FilePath:    opts.FilePath,
				Required:    opts.Required,
				Hidden:      opts.Hidden,
				Destination: any(opts.Destination).(*bool),
			}
		}
	default:
		fun.Invariant(out == nil, fmt.Sprintf("flag constructor for %T is not defined", opts.Default))
	}
	return nil
}

func listMenus() cli.Command {
	return cli.Command{
		Name: "list",
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()

			table := tabby.New()
			table.AddHeader("Name", "Selections")
			for _, m := range conf.Menus {
				table.AddLine(m.Name, strings.Join(m.Selections, ", "))
			}

			cmds := []string{}
			for _, cmd := range conf.TerminalCommands {
				cmds = append(cmds, cmd.Name)
			}
			table.AddLine("term", strings.Join(cmds, ", "))
			cmds = []string{}
			for _, cmd := range conf.Commands {
				cmds = append(cmds, cmd.Name)
			}
			table.AddLine("run", strings.Join(cmds, ", "))

			table.Print()
			return nil
		},
	}
}

func DMenu() cli.Command {
	cmds := dmenuListCmds(dmenuListCommandRun)
	cmds.Name = "run"

	term := dmenuListCmds(dmenuListCommandTerm)
	term.Name = "term"

	return cli.Command{
		Name: "dmenu",
		Subcommands: []cli.Command{
			cmds,
			term,
			listMenus(),
		},
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(commandFlagName, "c"),
				Usage: "specify a default flag name",
			},
		},
		Before: requireStringOrFirstArgSet(commandFlagName),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			name := c.String(commandFlagName)
			conf := env.Configuration()
			for _, menu := range conf.Menus {
				if menu.Name == name {
					opts := make([]string, len(menu.Selections))
					for idx := range opts {
						opts[idx] = menu.Selections[idx]
					}

					output, err := godmenu.RunDMenu(ctx, godmenu.Options{Selections: opts})
					if err != nil {
						return err
					}

					return env.Jasper().CreateCommand(ctx).Append(fmt.Sprintf("%s %s", menu.Command, output)).Run(ctx)
				}
			}

			return fmt.Errorf("could not find dmenu %q", name)
		},
	}
}
