package operations

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func listMenus() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				table := tabby.New()
				table.AddHeader("Name", "Selections")

				groups := map[string][]string{}

				for name, group := range conf.ExportCommandGroups() {
					cmds := []string{}
					for _, cmd := range group.Commands {
						switch {
						case cmd.Name != "":
							cmds = append(cmds, cmd.Name)
						case cmd.Alias != "":
							cmds = append(cmds, cmd.Alias)
						default:
							cmds = append(cmds, cmd.Command)
							for _, cg := range cmd.Commands {
								cmds = append(cmds, cg)
							}
						}
					}
					sort.Strings(cmds)
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
			}).Add)
}

func DMenu() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("dmenu").
		Subcommanders(
			dmenuListCmds(dmenuListCommandAll).SetName("all"),
			listMenus(),
		).
		Flags(cmdr.FlagBuilder("").
			SetName(commandFlagName, "c").
			SetUsage("specify a default flag name").
			Flag()).
		With(cmdr.SpecBuilder(ResolveConfiguration).SetMiddleware(sardis.WithConfiguration).Add).
		Middleware(sardis.WithDesktopNotify).
		With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (string, error) {
			if name := cc.String(commandFlagName); name != "" {
				return name, nil
			}

			return cc.Args().First(), nil
		}).SetAction(func(ctx context.Context, name string) error {
			conf := sardis.AppConfiguration(ctx)
			others := []string{}

			cmdGrp := conf.ExportCommandGroups()
			for group := range cmdGrp {
				others = append(others, group)
			}
			if group, ok := cmdGrp[name]; ok {
				cmds := group.ExportCommands()
				opts := make([]string, 0, len(cmds))
				for name, obj := range cmds {
					if (obj.Alias != "" && name == obj.Name) || name == "" {
						continue
					}

					opts = append(opts, name)
				}

				sort.Strings(opts)

				cmd, err := godmenu.RunDMenu(ctx, godmenu.Options{
					Selections: opts,
					DMenu:      conf.Settings.DMenu,
				})
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return err
				}

				return runConfiguredCommand(ctx, conf, []string{cmd})
			}

			notify := sardis.DesktopNotify(ctx)

			for _, menu := range conf.Menus {
				others = append(others, menu.Name)
				if menu.Name != name {
					continue
				}

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

				output, err := godmenu.RunDMenu(ctx, godmenu.Options{Selections: opts, DMenu: conf.Settings.DMenu})
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return err
				}

				var cmd string
				if menu.Command == "" {
					cmd = mapping[output]
				} else {
					cmd = fmt.Sprintf("%s %s", menu.Command, mapping[output])
				}

				err = jasper.Context(ctx).CreateCommand(ctx).
					AddEnv("SARDIS_LOG_QUIET_STDOUT", "true").
					Append(cmd).
					SetCombinedSender(level.Notice, grip.Sender()).
					Run(ctx)
				if err != nil {
					notify.Errorf("%s running %s failed: %s", name, output, err.Error())
					return err
				}
				notify.Noticef("%s running %s completed", name, output)
				return nil
			}
			sort.Strings(others)
			output, err := godmenu.RunDMenu(ctx, godmenu.Options{Selections: others, DMenu: conf.Settings.DMenu})
			switch {
			case err == nil:
				break
			case ers.Is(err, godmenu.ErrSelectionMissing):
				return nil
			default:
				return err
			}

			if output == "" {
				return errors.New("no selection")
			}

			// don't notify here let the inner one do that
			return jasper.Context(ctx).
				CreateCommand(ctx).
				AddEnv("SARDIS_LOG_QUIET_STDOUT", "true").
				Append(fmt.Sprintf("%s %s", "sardis dmenu", output)).
				SetCombinedSender(level.Notice, grip.Sender()).
				Run(ctx)
		}).Add)
}
