package repo

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis/subexec"
)

func (conf *Configuration) ConcreteTaskGroups() dt.Slice[subexec.Group] {

	pull := subexec.Group{
		Category: "repo",
		Name:     "pull",
	}

	update := subexec.Group{
		Category: "repo",
		Name:     "update",
	}

	for idx := range conf.GitRepos {
		repo := conf.GitRepos[idx]
		if repo.Disabled {
			continue
		}
		if !repo.Fetch && !repo.LocalSync {
			continue
		}

		pull.Commands = append(pull.Commands, subexec.Command{
			Name:             repo.Name,
			WorkerDefinition: repo.FetchJob(),
			Notify:           ft.Ptr(repo.Notify),
		})

		if repo.LocalSync {
			update.Commands = append(pull.Commands, subexec.Command{
				Name:             repo.Name,
				WorkerDefinition: repo.UpdateJob(),
				Notify:           ft.Ptr(repo.Notify),
			})
		}

		// TODO figure out why this err doesn't really work
		//      - implementation problem with the underlying operation
		//      - also need to wrap it in a pager at some level.
		//      - and disable syslogging
		// subexec.Command{
		// 	Name:            "status",
		// 	Directory:       repo.Path,
		// 	OverrideDefault: true,
		// 	Command:         "alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command sardis repo status {{prefix}}",
		// },
	}

	return []subexec.Group{pull, update}
}

func (conf *Configuration) SyntheticTaskGroups() dt.Slice[subexec.Group] {
	pull := subexec.Group{
		Category:      "repo",
		Name:          "pull",
		CmdNamePrefix: "tag",
		Notify:        ft.Ptr(true),
	}

	update := subexec.Group{
		Category:      "repo",
		Name:          "update",
		CmdNamePrefix: "tag",
		Notify:        ft.Ptr(true),
	}

	conf.rebuildIndexes()

	for tag, repos := range conf.caches.tags {
		var anyActive bool
		for _, rn := range repos {
			r, ok := conf.caches.lookup[rn]
			if !ok || r.Disabled || (r.Fetch == false && r.LocalSync == false) {
				continue
			}
			anyActive = true
			break
		}
		if !anyActive {
			continue
		}

		batch := dt.Slice[GitRepository]{}

		for _, name := range repos {
			repo, ok := conf.caches.lookup[name]
			if !ok {
				continue
			}

			batch.Add(repo)
		}

		pull.Commands = append(pull.Commands, subexec.Command{
			Name: tag,
			WorkerDefinition: func(ctx context.Context) error {
				return fun.MakeConverter(func(r GitRepository) fun.Worker {
					return r.FetchJob()
				}).Stream(batch.Stream()).Parallel(
					func(ctx context.Context, op fun.Worker) error { return op(ctx) },
					fun.WorkerGroupConfContinueOnError(),
					fun.WorkerGroupConfWorkerPerCPU(),
				).Run(ctx)
			},
		})

		update.Commands = append(update.Commands, subexec.Command{
			Name: tag,
			WorkerDefinition: func(ctx context.Context) error {
				return fun.MakeConverter(func(r GitRepository) fun.Worker {
					return r.UpdateJob()
				}).Stream(batch.Stream()).Parallel(
					func(ctx context.Context, op fun.Worker) error { return op(ctx) },
					fun.WorkerGroupConfContinueOnError(),
					fun.WorkerGroupConfWorkerPerCPU(),
				).Run(ctx)
			},
		})
	}

	return []subexec.Group{pull, update}
}
