package operations

import (
	"fmt"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func RunCommand() cli.Command {
	const commandFlagName = "command"
	return cli.Command{
		Name: "run",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(commandFlagName, "c"),
				Usage: "specify a default flag name",
			},
		},
		Before: mergeBeforeFuncs(
			requireConfig(),
			requireCommandsSet(commandFlagName),
		),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			defer env.Close(ctx)
			conf := env.Configuration()

			cmds := conf.ExportCommands()
			ops := c.StringSlice(commandFlagName)
			for idx, name := range ops {
				cmd, ok := cmds[name]
				if !ok {
					return errors.Errorf("command '%s' [%d/%d] does not exist", name, idx+1, len(ops))
				}
				err := env.Jasper().CreateCommand(ctx).Directory(cmd.Directory).ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(ops))).
					Append(cmd.Command).SetCombinedSender(level.Info, grip.GetSender()).
					Prerequisite(func() bool {
						grip.Info(message.Fields{
							"cmd": name,
							"dir": cmd.Directory,
							"num": idx + 1,
							"len": len(ops),
						})
						return true
					}).Run(ctx)
				if err != nil {
					return errors.WithStack(err)
				}
			}

			return nil
		},
	}

}
