package repo

import (
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/sardis/subexec"
)

func (conf *Configuration) ConcreteTaskGroups() stw.Slice[subexec.Group] {
	pull := subexec.Group{
		Category:  "repo",
		Name:      "pull",
		Synthetic: true,
		SortHint:  -8,
	}

	update := subexec.Group{
		Category:  "repo",
		Name:      "update",
		Synthetic: true,
		SortHint:  16,
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
			Notify:           stw.Ptr(repo.Notify),
			SortHint:         -4,
		})

		if repo.LocalSync {
			update.Commands = append(update.Commands, subexec.Command{
				Name:             repo.Name,
				WorkerDefinition: repo.UpdateJob(),
				Notify:           stw.Ptr(repo.Notify),
				SortHint:         16,
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

	return []subexec.Group{update, pull}
}

func (conf *Configuration) SyntheticTaskGroups() stw.Slice[subexec.Group] {
	pull := subexec.Group{
		Category:      "repo",
		Name:          "pull",
		CmdNamePrefix: "tag",
		Notify:        stw.Ptr(true),
		Synthetic:     true,
		SortHint:      -4,
	}

	update := subexec.Group{
		Category:      "repo",
		Name:          "update",
		CmdNamePrefix: "tag",
		Notify:        stw.Ptr(true),
		Synthetic:     true,
		SortHint:      16,
	}

	conf.rebuildIndexes()

	for tag, repos := range conf.caches.tags {
		var anyActive bool
		for _, rn := range repos {
			r, ok := conf.caches.lookup[rn]
			if !ok || r.Disabled || (!r.Fetch && !r.LocalSync) {
				continue
			}
			anyActive = true
			break
		}
		if !anyActive {
			continue
		}

		batch := stw.Slice[GitRepository]{}

		for _, name := range repos {
			repo, ok := conf.caches.lookup[name]
			if !ok {
				continue
			}

			batch.Push(repo)
		}

		pull.Commands = append(pull.Commands, subexec.Command{
			Name:     tag,
			SortHint: -4,
			WorkerDefinition: wpa.RunWithPool(
				irt.Convert(
					irt.Slice(batch),
					func(r GitRepository) fnx.Worker {
						return r.FetchJob()
					},
				),
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			),
		})

		update.Commands = append(update.Commands, subexec.Command{
			Name:     tag,
			SortHint: 8,
			WorkerDefinition: wpa.RunWithPool(
				irt.Convert(
					irt.Slice(batch),
					func(r GitRepository) fnx.Worker {
						return r.UpdateJob()
					},
				),
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			),
		})
	}

	return []subexec.Group{update, pull}
}
