package repo

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis/subexec"
)

func (conf *Configuration) ConcreteTaskGroups() dt.Slice[subexec.Group] {
	grps := make([]subexec.Group, 0, len(conf.GitRepos))

	for idx := range conf.GitRepos {
		repo := conf.GitRepos[idx]
		if repo.Disabled {
			continue
		}
		if !repo.Fetch && !repo.LocalSync {
			continue
		}

		cg := subexec.Group{
			Name:   "repo",
			Notify: ft.Ptr(repo.Notify),
			Commands: []subexec.Command{
				subexec.Command{
					Name:             fmt.Sprint("pull.", repo.Name),
					WorkerDefinition: repo.FetchJob(),
				},
				// TODO figure out why this err doesn't really work
				//      - implementation problem with the underlying operation
				//      - also need to wrap it in a pager at some level.
				//      - and disable syslogging
				// {
				// 	Name:            "status",
				// 	Directory:       repo.Path,
				// 	OverrideDefault: true,
				// 	Command:         "alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command sardis repo status {{prefix}}",
				// },
			},
		}

		if repo.LocalSync {
			cg.Commands = append(cg.Commands, subexec.Command{
				Name:             fmt.Sprint("update.", repo.Name),
				WorkerDefinition: repo.UpdateJob(),
			})
		}

		grps = append(grps, cg)
	}

	return grps
}

func (conf *Configuration) SyntheticTaskGroups() dt.Slice[subexec.Group] {
	grps := make([]subexec.Group, 0, len(conf.GitRepos))

	repoTags := conf.Tags()
	for tag, repos := range repoTags {
		anyActive := false
		for _, r := range repos {
			if r.Disabled || (r.Fetch == false && r.LocalSync == false) {
				continue
			}
			anyActive = true
			break
		}
		if !anyActive {
			continue
		}

		tagName := tag
		opName := fmt.Sprint("tagged.", tagName)
		grps = append(grps, subexec.Group{
			Name:   "repo",
			Notify: ft.Ptr(true),
			Commands: []subexec.Command{
				{
					Name: fmt.Sprint("pull.", opName),
					WorkerDefinition: func(ctx context.Context) error {
						repos := fun.MakeConverter(func(r *GitRepository) fun.Worker {
							return r.FetchJob()
						}).Stream(repoTags.Get(tagName).Stream())

						return repos.Parallel(
							func(ctx context.Context, op fun.Worker) error { return op(ctx) },
							fun.WorkerGroupConfContinueOnError(),
							fun.WorkerGroupConfWorkerPerCPU(),
						).Run(ctx)
					},
				},
				{
					Name: fmt.Sprint("update.", opName),
					WorkerDefinition: func(ctx context.Context) error {
						repos := fun.MakeConverter(func(r *GitRepository) fun.Worker {
							return r.UpdateJob()
						}).Stream(repoTags.Get(tagName).Stream())

						return repos.Parallel(
							func(ctx context.Context, op fun.Worker) error { return op(ctx) },
							fun.WorkerGroupConfContinueOnError(),
							fun.WorkerGroupConfWorkerPerCPU(),
						).Run(ctx)
					},
				},
			},
		})
	}
	return grps
}
