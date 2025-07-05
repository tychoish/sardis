package operations

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

				for name, group := range conf.ExportCommandGroups() {
					cmds := []string{}
					for _, cmd := range group.Commands {
						if cmd.Name == "" && len(cmd.Aliases) == 0 {
							cmds = append(cmds, cmd.Command)
							for _, cg := range cmd.Commands {
								cmds = append(cmds, cg)
							}
							continue
						}
						cmds = append(cmds, cmd.Name)
						cmds = append(cmds, cmd.Aliases...)
					}
					if len(cmds) == 0 {
						grip.Debugf("skipping empty command group %q", name)
						continue
					}
					idx := -1
					for chunk := range slices.Chunk(cmds, 4) {
						idx++
						if idx == 0 {
							table.AddLine(name, strings.Join(chunk, "; "))
						} else {
							table.AddLine("", strings.Join(chunk, "; "))
						}
					}
					table.AddLine("", "")
				}

				for _, m := range conf.Menus {
					if len(m.Selections) == 0 {
						grip.Debugf("skipping empty menu %q", m.Name)
						continue
					}

					idx := -1
					for chunk := range slices.Chunk(m.Selections, 4) {
						idx++
						if idx == 0 {
							table.AddLine(m.Name, strings.Join(chunk, "; "))
						} else {
							table.AddLine("", strings.Join(chunk, "; "))
						}
					}
					table.AddLine("", "")
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
			dmenuListCmds(dmenuListCommandGroup).SetName("groups"),
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
			cmdGrp := conf.ExportCommandGroups()
			dmenuConf := conf.Settings.DMenu
			// if we're running "sardis dmenu <group>" and
			// the group exists:
			if group, ok := cmdGrp[name]; ok {
				opts := make([]string, 0, len(group.Commands))
				for _, cmd := range group.Commands {
					opts = append(opts, cmd.Name)
				}

				sort.Strings(opts)

				cmd, err := godmenu.Do(ctx, godmenu.Options{
					Selections: opts,
					Flags:      &dmenuConf,
				})
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return ers.Wrapf(err, "group %q", name)
				}
				ops, err := getcmds(conf, group.Commands, []string{cmd})
				if err != nil {
					return err
				}

				return runConfiguredCommand(ctx, conf, ops)
			}

			notify := sardis.DesktopNotify(ctx)
			for _, menu := range conf.Menus {
				if menu.Name != name {
					continue
				}

				mapping := make(map[string]string, len(menu.Selections))
				opts := make([]string, 0, len(menu.Selections))
				for _, item := range menu.Selections {
					opts = append(opts, item)
					mapping[item] = item
				}

				output, err := godmenu.Do(ctx, godmenu.Options{Selections: opts, Flags: &dmenuConf})
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return ers.Wrapf(err, "menu %q", name)
				}

				var cmd string
				if menu.Command == "" {
					cmd = mapping[output]
				} else {
					cmd = fmt.Sprintf("%s %s", menu.Command, mapping[output])
				}

				err = jasper.Context(ctx).CreateCommand(ctx).
					AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
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

			// build a list of all the groups and menu selectors to add to
			// the "fallback option""
			others := make([]string, 0, len(cmdGrp)+len(conf.Menus))
			for group := range cmdGrp {
				if group == "" {
					continue
				}
				others = append(others, group)
			}
			for _, menu := range conf.Menus {
				if menu.Name == "" {
					continue
				}
				others = append(others, menu.Name)
			}
			sort.Strings(others)

			output, err := godmenu.Do(ctx, godmenu.Options{Selections: others, Flags: &dmenuConf})
			switch {
			case err == nil:
				break
			case ers.Is(err, godmenu.ErrSelectionMissing):
				return nil
			default:
				return ers.Wrapf(err, "top-level %q", name)
			}

			if output == "" {
				return errors.New("no selection")
			}

			// don't notify here let the inner one do that
			return jasper.Context(ctx).
				CreateCommand(ctx).
				AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
				AppendArgs("sardis", "dmenu", output).
				SetCombinedSender(level.Notice, grip.Sender()).
				Run(ctx)
		}).Add)
}
