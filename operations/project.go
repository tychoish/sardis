package operations

import (
	"os"
	"strings"

	"github.com/deciduosity/utility"
	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Project() cli.Command {
	return cli.Command{
		Name:  "project",
		Usage: "commands for managing groups of repositories",
		Subcommands: []cli.Command{
			projectStatus(),
			projectClone(),
			projectFetch(),
		},
	}
}

func projectStatus() cli.Command {
	const projectFlagName = "project"
	return cli.Command{
		Name:  "status",
		Usage: "get the status of all repositories in a project",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(projectFlagName, "p"),
				Usage: "specify the name of a project to get its status",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(), setAllTailArguements(projectFlagName)),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			projs := getProjects(c.StringSlice(projectFlagName), env.Configuration())
			if len(projs) == 0 {
				return errors.New("no configured matching projects")
			}

			for idx := range projs {
				if err := amboy.RunJob(ctx, units.NewProjectStatusJob(projs[idx])); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func projectFetch() cli.Command {
	const projectFlagName = "project"
	return cli.Command{
		Name:  "fetch",
		Usage: "updates (git pull) all repos in a project",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(projectFlagName, "p"),
				Usage: "specify the name of a project to get its status",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(), setAllTailArguements(projectFlagName)),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			projs := getProjects(c.StringSlice(projectFlagName), env.Configuration())
			if len(projs) == 0 {
				return errors.New("no configured matching projects")
			}

			for idx := range projs {
				if err := amboy.RunJob(ctx, units.NewProjectFetchJob(projs[idx])); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func projectClone() cli.Command {
	const projectFlagName = "project"
	return cli.Command{
		Name:  "clone",
		Usage: "clones all repositories in a project",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(projectFlagName, "p"),
				Usage: "specify the name of a project to get its status",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(), setAllTailArguements(projectFlagName)),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			projs := getProjects(c.StringSlice(projectFlagName), env.Configuration())
			if len(projs) == 0 {
				return errors.New("no configured matching projects")
			}

			for idx := range projs {
				if err := amboy.RunJob(ctx, units.NewProjectCloneJob(projs[idx])); err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func getProjects(args []string, conf *sardis.Configuration) []sardis.ProjectConf {
	if len(args) == 0 {
		cwd, _ := os.Getwd()
		if cwd == "" {
			return nil
		}
		for idx, proj := range conf.Projects {
			if strings.HasPrefix(cwd, proj.Options.Directory) || proj.Options.Directory == cwd {
				return []sardis.ProjectConf{conf.Projects[idx]}
			}
		}
		return nil
	}

	out := []sardis.ProjectConf{}
	for idx, proj := range conf.Projects {
		if utility.StringSliceContains(args, proj.Name) {
			out = append(out, conf.Projects[idx])
		}
	}

	return out
}
