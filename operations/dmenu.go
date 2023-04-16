package operations

import (
	"fmt"
	"strings"

	"github.com/cheynewallace/tabby"
	"github.com/tychoish/fun"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
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
	case int64:
		out = cli.Int64Flag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(int64),
			Destination: any(opts.Destination).(*int64),
		}
	case float64:
		out = cli.Float64Flag{
			Name:        opts.Name,
			Usage:       opts.Usage,
			EnvVar:      opts.EnvVar,
			FilePath:    opts.FilePath,
			Required:    opts.Required,
			Hidden:      opts.Hidden,
			Value:       any(opts.Default).(float64),
			Destination: any(opts.Destination).(*float64),
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
	case []int64:
		o := cli.Int64SliceFlag{
			Name:     opts.Name,
			Usage:    opts.Usage,
			EnvVar:   opts.EnvVar,
			FilePath: opts.FilePath,
			Required: opts.Required,
			Hidden:   opts.Hidden,
		}
		if opts.Destination != nil {
			vd := any(opts.Destination).(*cli.Int64Slice)
			o.Value = vd
		} else {
			vd := cli.Int64Slice(any(opts.Default).([]int64))
			o.Value = &vd
		}

		out = o
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

			cmds := []string{}
			groups := map[string][]string{}
			for _, cmd := range conf.TerminalCommands {
				if cmd.Group == "" {
					cmds = append(cmds, cmd.Name)
					continue
				}
				groups[cmd.Group] = append(groups[cmd.Group], cmd.Name)
			}
			if len(cmds) > 0 {
				table.AddLine("term", strings.Join(cmds, ", "))
			}
			cmds = []string{}
			for _, cmd := range conf.Commands {
				if cmd.Group == "" {
					cmds = append(cmds, cmd.Name)
					continue
				}
				groups[cmd.Group] = append(groups[cmd.Group], cmd.Name)
			}
			if len(cmds) > 0 {
				table.AddLine("run", strings.Join(cmds, ", "))
			}
			for _, m := range conf.Menus {
				out := make([]string, 0, len(m.Selections)+len(m.Aliases))
				out = append(out, m.Selections...)
				if len(m.Aliases) > 0 {
					for _, p := range m.Aliases {
						out = append(out, fmt.Sprintf("%s [%s]", p.Key, p.Value))
					}
				}
				if len(out) > 0 {
					table.AddLine(m.Name, strings.Join(out, "; "))
				}
			}

			for group, cmds := range groups {
				table.AddLine(group, strings.Join(cmds, ", "))
			}

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
				Name:     joinFlagNames(commandFlagName, "c"),
				Usage:    "specify a default flag name",
				Required: false,
			},
		},
		Before: setFirstArgWhenStringUnset(commandFlagName),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			name := c.String(commandFlagName)
			conf := env.Configuration()
			others := []string{}
			groupings := map[string][]string{}
			for _, cmd := range append(conf.Commands, conf.TerminalCommands...) {
				if cmd.Group == "" {
					continue
				}
				groupings[cmd.Group] = append(groupings[cmd.Group], cmd.Name)
			}

			if group, ok := groupings[name]; ok {
				cmd, err := godmenu.RunDMenu(ctx, godmenu.Options{
					Selections: group,
				})

				if err != nil {
					return err
				}

				return runConfiguredCommand(ctx, env, []string{cmd})
			}

			for _, menu := range conf.Menus {
				if menu.Name == name {
					items := len(menu.Selections) + len(menu.Aliases)
					mapping := make(map[string]string, len(menu.Selections)+len(menu.Aliases))
					opts := make([]string, 0, items)
					for _, item := range opts {
						opts = append(opts, item)
						mapping[item] = item
					}
					for _, p := range menu.Aliases {
						mapping[p.Key] = p.Value
						opts = append(opts, p.Key)
					}

					output, err := godmenu.RunDMenu(ctx, godmenu.Options{Selections: opts})
					if err != nil {
						return err
					}
					var cmd string
					if menu.Command == "" {
						cmd = mapping[output]
					} else {
						cmd = fmt.Sprintf("%s %s", menu.Command, mapping[output])
					}

					err = env.Jasper().CreateCommand(ctx).Append(cmd).
						SetCombinedSender(level.Notice, grip.Sender()).Run(ctx)
					if err != nil {
						env.Logger().Errorf("%s running %s failed: %s", name, output, err.Error())
						return err
					}
					env.Logger().Noticef("%s running %s completed", name, output)
					return nil
				}
				others = append(others, menu.Name)
			}
			for group := range groupings {
				others = append(others, group)
			}

			others = append(others, "term", "run")
			output, err := godmenu.RunDMenu(ctx, godmenu.Options{Selections: others})
			if err != nil {
				return err
			}
			// don't notify here let the inner one do that
			return env.Jasper().CreateCommand(ctx).Append(fmt.Sprintf("%s %s", "sardis dmenu", output)).
				SetCombinedSender(level.Notice, grip.Sender()).Run(ctx)

		},
	}
}
