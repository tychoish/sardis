package operations

import (
	"bufio"
	"context"
	"fmt"
	"iter"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/cheynewallace/tabby"
	fzf "github.com/koki-develop/go-fzf"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/global"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/tools/execpath"
	"github.com/tychoish/sardis/util"
)

const commandFlagName string = "command"

func RunCommand() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("run").
		Aliases("r").
		SetUsage("runs a predefined command").
		Subcommanders(
			listCommands(),
		),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			cmds, err := subexec.FilterCommands(args.conf.Operations.ExportAllCommands(), args.arg)
			if err != nil {
				return ers.Wrapf(err, "resolving commands %s", args.arg)
			}

			return subexec.RunCommands(ctx, cmds)
		})
}

func SearchMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("cmd").
		SetUsage("list or run a command").
		Aliases("c", "m", "cmds").
		Subcommanders(
			fuzzy(),
			searchCommand(),
			listCommands(),
		),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			if args.conf.Settings.Runtime.WithAnnotations {
				fun.Invariant.Ok(len(args.conf.Settings.Runtime.AnnotationSeparator) > 0,
					"annotation separator must be defined as something other than the empty string.")

				fun.Invariant.Ok(args.conf.Settings.Runtime.AnnotationSeparator != "\n",
					"annotation separator must be defined as something other than a newline character.")

				for idx, op := range args.arg {
					if op == "" {
						continue
					}
					args.arg[idx] = strings.SplitN(op, args.conf.Settings.Runtime.AnnotationSeparator, 1)[0]
				}
			}

			stage, err := args.conf.Operations.ResolveCommands(args.arg)
			var ops []string

			switch {
			case err != nil:
				return err
			case stage.Commands != nil:
				return subexec.RunCommands(ctx, stage.Commands)
			case stage.Prefixed != nil:
				ops = stage.Prefixed
			case stage.Selections != nil:
				ops = stage.Selections
			}

			if args.conf.Settings.Runtime.WithAnnotations {
				index := args.conf.Operations.Tree()
				for idx, op := range ops {
					cmd := index.FindCommand(op)
					ops[idx] = fmt.Sprint(op, " ", args.conf.Settings.Runtime.AnnotationSeparator, " ", cmd.Command)
					if len(cmd.Commands) > 0 {
						ops[idx] = fmt.Sprintf("%s ... +%d", ops[idx], len(cmd.Commands))
					}
				}
			}

			buf := bufio.NewWriter(os.Stdout)
			for _, op := range ops {
				ft.Ignore(ft.Must(fmt.Fprintln(buf, op)))
			}

			return buf.Flush()
		},
	)
}

func ExecCommand() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("exec").
		SetUsage("list or run a command"),
		"command", func(ctx context.Context, args *withConf[string]) error {
			st, err := itertool.Uniq(execpath.FindAll(ctx)).Slice(ctx)

			res, err := godmenu.Run(ctx,
				godmenu.SetSelections(st),
				godmenu.WithFlags(ft.Ptr(args.conf.Settings.DMenuFlags)),
				godmenu.Prompt(fmt.Sprintf("%s exec ==>>", "sardis")),
				godmenu.MenuLines(min(len(st), args.conf.Settings.DMenuFlags.Lines)),
			)
			if err != nil {
				return err
			}

			if strings.Contains(res, " ") {
				return jasper.Context(ctx).CreateCommand(ctx).ShellScript("bash", res).Run(ctx)
			}
			return jasper.Context(ctx).CreateCommand(ctx).Append(res).Run(ctx)
		},
	)
}

func searchCommand() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("search").
		SetUsage("list or run a command").
		Aliases("s", "find", "f").
		Subcommanders(
			listCommands(),
			fuzzy(),
		),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			prefix := util.DotJoinParts(args.arg)
			searchTree := args.conf.Operations.Tree().Find(prefix)

			var options []string
			switch {
			case searchTree == nil:
				return fmt.Errorf("no command found with prefix %q", prefix)
			case searchTree.HasCommand() && searchTree.HasChidren():
				cmd := searchTree.Command()

				// hopefully logging for this all goes to standard err and not stdout 😬
				if err := subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{cmd})); err != nil {
					return fmt.Errorf("problem running command %s, %w; missed running children %s", cmd.Name, err, prefix)
				}
			case searchTree.HasCommand():
				return subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{searchTree.Command()}))
			case !searchTree.HasChidren():
				return fmt.Errorf("no further selections at %q", prefix)
			}

			options = searchTree.KeysAtLevel()
			sort.Strings(options)
			slices.Sort(options)

			buf := bufio.NewWriter(os.Stdout)
			for op := range slices.Values(options) {
				ft.Ignore(ft.Must(fmt.Fprintln(buf, util.DotJoin(prefix, op))))
			}

			return buf.Flush()
		},
	)
}

func fuzzy() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fuzzy").
			Aliases("fuzz", "fzf", "fz", "ff").
			Subcommanders(
				fuzzySearch(),
			),
		"name",
		func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg

			prompt := new(dt.List[string])
			prompt.PushBack(util.GetHostname())
			prompt.PushBack("sardis")

			opr := GetOperationRuntime(ctx)
			for {
				stage, err := args.conf.Operations.ResolveCommands(op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					err, ranFor := util.DoWithTiming(func() error { return subexec.RunCommands(ctx, stage.Commands) })

					waitedFor := util.CallWithTiming(func() {
						if opr.ShouldBlock && err == nil {
							<-ctx.Done()
						}
					})

					grip.Notice(message.BuildPair().
						Pair("op", "cmd.fuzzy").
						Pair("state", "COMPLETED").
						Pair("err", err != nil).
						Pair("waited", opr.ShouldBlock).
						Pair("run_dur", ranFor.Span()).
						Pair("wait_dur", waitedFor.Span()).
						Pair("commands", strings.Join(stage.CommandNames(), ", ")))

					return err
				case stage.Selections != nil:
					pv := util.DotJoinParts(prompt.Slice())
					idxs, err := ft.Must(fzf.New(
						fzf.WithPrompt(fmt.Sprintf("%s =>> ", pv)),
						fzf.WithNoLimit(true),
						fzf.WithCaseSensitive(false),
					)).Find(stage.Selections, stage.SelectionAt)
					if err != nil {
						return fmt.Errorf("fuzzy selection [%s]: %w", pv, err)
					}

					op = stage.Resolve(idxs)

					switch len(op) {
					case 0:
						return ers.New("not found")
					case 1:
						prompt.PushBack(op[0])
					default:
						prompt.PushBack(fmt.Sprintf("[%s]", strings.Join(op, ",")))
					}

				default:
					// this should be impossible
					return ers.Error("unexpect outcome")
				}
			}
		})
}

func fuzzySearch() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("search").
			Aliases("find", "s", "f"),
		"name",
		func(ctx context.Context, args *withConf[[]string]) error {
			searchTree := args.conf.Operations.Tree()

			for ct := 0; true; ct++ {
				switch {
				case searchTree == nil:
					return fmt.Errorf("no command found at level %d, ", ct)
				case searchTree.HasCommand() && searchTree.HasChidren():
					if err := subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{searchTree.Command()})); err != nil {
						return err
					}
				case searchTree.HasCommand():
					return subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{searchTree.Command()}))
				case !searchTree.HasChidren():
					return fmt.Errorf("no further selections at level %d", ct)
				}

				selections := searchTree.KeysAtLevel()
				selected, err := ft.Must(fzf.New(
					fzf.WithPrompt(fmt.Sprintf("%s.%s ==> ", util.GetHostname(), global.ApplicationName)),
					fzf.WithNoLimit(true),
					fzf.WithCaseSensitive(false),
				)).Find(selections, func(id int) string { return selections[id] })
				if err != nil {
					return fmt.Errorf("fuzzy search selections [%s]: %w", searchTree.ID(), err)
				}

				cmds := []*subexec.Command{}
				nextSearch := subexec.NewNode()

				for _, sidx := range selected {
					id := selections[sidx]
					sn := searchTree.NarrowTo(id)
					fun.Invariant.Ok(sn != nil, "cannot resolve nil nodes in the search")

					cmds = append(cmds, sn.Command())
					nextSearch.Extend(sn.Children())
				}

				searchTree = nextSearch
				if len(cmds) > 0 {
					if err := subexec.RunCommands(ctx, slices.Collect(util.MakeSparseRefs(slices.Values(cmds)))); err != nil {
						return err
					}
					if searchTree.Len() == 0 {
						return nil
					}
				}
			}
			panic("unreachable")
		})
}

func DMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("dmenu").
		Aliases("d", "menu").
		SetUsage("unless running a subcommand, launches a menu for specific group specific group, or attmepts to run a command directly.").
		Subcommanders(
			dmenuSearch(),
			listCommands(),
		),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg
			var selected string

			for {
				stage, err := args.conf.Operations.ResolveCommands(op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					return subexec.RunCommands(ctx, stage.Commands)
				case stage.Selections != nil:
					selected, err = godmenu.Run(ctx,
						godmenu.SetSelections(stage.Selections),
						godmenu.WithFlags(ft.Ptr(args.conf.Settings.DMenuFlags)),
						godmenu.Prompt(fmt.Sprintf("%s ==>>", ft.Default(stage.NextLabel, "sardis"))),
						godmenu.MenuLines(min(len(stage.Selections), args.conf.Settings.DMenuFlags.Lines)),
					)

					switch {
					case err != nil && ers.Is(err, godmenu.ErrSelectionMissing):
						return nil
					case err != nil:
						return err
					default:
						op = []string{util.DotJoin(stage.Prefix, selected)}
					}
				default:
					return ers.Error("unexpect outcome")
				}
			}
		})
}

func dmenuSearch() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("search").
			Aliases("find", "s", "f"),
		"name",
		func(ctx context.Context, args *withConf[string]) error {
			path := new(dt.List[string])

			searchTree := args.conf.Operations.Tree()
			if args.arg != "" {
				path.PushBack(args.arg)
				searchTree = searchTree.NarrowTo(args.arg)
			}

			prompt := "sardis.root"
			for {
				switch {
				case searchTree == nil:
					return fmt.Errorf("no command found named %s []", util.DotJoin(path.Slice()...))
				case searchTree.HasCommand() && searchTree.HasChidren():
					if path.Len() > 0 && searchTree.ID() == path.Back().Value() {
						if err := subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{searchTree.Command()})); err != nil {
							return err
						}
					}
				case searchTree.HasCommand():
					return subexec.RunCommands(ctx, dt.SliceRefs([]*subexec.Command{searchTree.Command()}))
				case !searchTree.HasChidren():
					return fmt.Errorf("no further selections at %s", util.DotJoin(path.Slice()...))
				}

				if path.Len() > 0 {
					prompt = path.Back().Value()
				}

				selections := searchTree.KeysAtLevel()
				selected, err := godmenu.Run(ctx,
					godmenu.Sorted(),
					godmenu.SetSelections(selections),
					godmenu.WithFlags(ft.Ptr(args.conf.Settings.DMenuFlags)),
					godmenu.Prompt(fmt.Sprintf("%s =>>", prompt)),
					godmenu.MenuLines(min(len(selections), args.conf.Settings.DMenuFlags.Lines)),
				)

				switch {
				case err != nil && ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				case err != nil:
					return err
				default:
					path.PushBack(selected)
					searchTree = searchTree.NarrowTo(selected)
				}
			}
		})
}

func listCommands() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("list").
			Aliases("ls", "l").
			Subcommanders(
				listCommandsWithInfo(),
				listCommandsPlain(),
			).
			SetUsage("prints all commands, group, and aliases."),
		"group", func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf
			set := dt.NewSetFromSlice(args.arg)

			groups := conf.Operations.ExportCommandGroups()
			groupSet := &dt.Set[string]{}
			groupSet.AppendStream(groups.Keys())

			table := tabby.New()
			table.AddHeader("Category", "Group", "Prefix", "Name")

			for name, group := range groups {
				if set.Len() == 0 || set.Check(name) {
					for idx, cc := range group.Commands {
						if idx == 0 {
							table.AddLine(group.Category, group.Name, group.CmdNamePrefix, cc.Name)
						} else {
							table.AddLine("", "", "", cc.Name)
						}
					}
					table.AddLine("", "", "", "")
				}
			}

			table.Print()
			return nil
		})
}

func listCommandsPlain() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("plain").
		Aliases("p", "pl").
		SetUsage("return a simple printed list of commands.").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				stage, err := conf.Operations.ResolveCommands(nil)

				switch {
				case err != nil:
					return err
				case len(stage.Selections) != 0:
					return fmt.Errorf("found invalid selections %s", stage.Selections)
				case len(stage.Commands) == 0:
					return ers.Error("no commands defined")
				}

				var ops iter.Seq[string]
				if len(stage.Prefixed) != 0 {
					ops = slices.Values(stage.Prefixed)
				} else {
					ops = slices.Values(stage.Selections)
				}

				buf := bufio.NewWriter(os.Stdout)

				for op := range ops {
					ft.Ignore(ft.Must(fmt.Fprintln(buf, op)))
				}

				return buf.Flush()
			}).Add)
}

func listCommandsWithInfo() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("info").
		Aliases("extra", "x", "shell").
		SetUsage("return a list of defined commands").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				homedir := util.GetHomeDir()

				table := tabby.New()
				table.AddHeader("Name", "Command", "Directory")
				for _, cmd := range conf.Operations.ExportAllCommands() {
					if cmd.Directory == homedir {
						cmd.Directory = ""
					}

					cmds := append([]string{cmd.Command}, cmd.Commands...)

					for idx := range cmds {
						if maxLen := 48; len(cmds[idx]) > maxLen {
							cmds[idx] = fmt.Sprintf("<%s...>", cmds[idx][:maxLen])
						}
					}

					for idx, chunk := range cmds {
						if idx == 0 {
							table.AddLine(
								cmd.Name,                               // name
								chunk,                                  // command
								util.TryCollapseHomeDir(cmd.Directory), //  dir
							)
						} else {
							table.AddLine(
								"",    // group
								"",    // names
								chunk, // command
								"",    // dir
							)
						}
					}
				}

				table.Print()

				return nil
			}).Add)
}
