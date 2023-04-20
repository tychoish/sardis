package operations

import (
	"fmt"
	"strings"

	"github.com/cheynewallace/tabby"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func listMenus() cli.Command {
	return cli.Command{
		Name: "list",
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()

			table := tabby.New()
			table.AddHeader("Name", "Selections")

			groups := map[string][]string{}

			for name, group := range conf.ExportCommandGroups() {
				cmds := []string{}
				for _, cmd := range group.Commands {
					switch {
					case cmd.Alias != "":
						cmds = append(cmds, cmd.Alias)
					case cmd.Name != "":
						cmds = append(cmds, cmd.Name)
					default:
						cmds = append(cmds, cmd.Command)
					}
				}
				table.AddLine(name, strings.Join(cmds, "; "))
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
	cmds.Name = "all"

	return cli.Command{
		Name: "dmenu",
		Subcommands: []cli.Command{
			cmds,
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

			cmdGrp := conf.ExportCommandGroups()
			for group := range cmdGrp {
				others = append(others, group)
			}

			if group, ok := cmdGrp[name]; ok {
				cmds := group.ExportCommands()
				opts := make([]string, 0, len(cmds))
				for name, obj := range cmds {
					if obj.Alias != "" && name == obj.Name {
						continue
					}

					opts = append(opts, name)
				}

				cmd, err := godmenu.RunDMenu(ctx, godmenu.Options{
					Selections: opts,
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
					for _, item := range menu.Selections {
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
