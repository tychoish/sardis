package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
)

func NewRepoStatusJob(path string) fun.Worker {
	return func(ctx context.Context) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("cannot check status %s, no repository exists", path)
		}

		cmd := jasper.Context(ctx)

		startAt := time.Now()

		logger := grip.Context(ctx)
		sender := logger.Sender()

		ec := &erc.Collector{}
		ec.Add(cmd.CreateCommand(ctx).Priority(level.Debug).
			Directory(path).
			SetOutputSender(level.Debug, sender).
			SetErrorSender(level.Debug, sender).
			Add(getStatusCommandArgs(path)).
			AppendArgs("git", "status", "--short", "--branch").
			Run(ctx))

		ec.Add(doOtherStat(path, logger))

		logger.Debug(message.Fields{
			"op":   "git status",
			"path": path,
			"secs": time.Since(startAt).Seconds(),
			"ok":   ec.Ok(),
		})

		return ec.Resolve()
	}
}

func doOtherStat(path string, logger grip.Logger) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	stat, err := wt.Status()
	if err != nil {
		return err
	}

	for fn, status := range stat {
		logger.Notice(message.Fields{
			"file":     fn,
			"stat":     "golib",
			"staging":  status.Staging,
			"worktree": status.Worktree,
		})
	}
	return nil
}

func getStatusCommandArgs(path string) []string {
	return []string{
		"git", "log", "--date=relative", "--decorate", "-n", "1",
		fmt.Sprintf("--format=%s:", filepath.Base(path)) + `%N (%cr) "%s"`,
	}

}
