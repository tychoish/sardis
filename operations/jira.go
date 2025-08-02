package operations

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/x/jira"
	"github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/util"
)

func Jira() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("jira").
		SetUsage("a collections of commands for jira management").
		Subcommanders(
			bulkCreateTickets(),
		)
}

func bulkCreateTickets() *cmdr.Commander {
	const pathFlagName = "path"

	return addOpCommand(
		cmdr.MakeCommander().
			SetName("create").
			SetUsage("create tickets as specified in a file"),
		pathFlagName, func(ctx context.Context, op *withConf[string]) error {
			path := ft.DefaultFuture(op.arg, func() string { return ft.Must(os.Getwd()) })

			if !util.FileExists(path) {
				return fmt.Errorf("ticket spec file file %s does not exist", path)
			}

			if op.conf.Settings.Credentials.Jira.URL == "" {
				return errors.New("cannot create jira tickets without jira url")
			}

			data := struct {
				Priority level.Priority `bson:"priority" json:"priority" yaml:"priority"`
				Tickets  []jira.Issue   `bson:"tickets" json:"tickets" yaml:"tickets"`
			}{}

			err := util.UnmarshalFile(path, &data)
			if err != nil {
				return err
			}

			data.Priority = level.Info

			ctx = srv.WithJiraIssue(ctx, op.conf.Settings)
			logger := srv.JiraIssue(ctx)

			ec := &erc.Collector{}

			msgs := make([]message.Composer, len(data.Tickets))

			for idx := range data.Tickets {
				msg := jira.MakeIssue(&data.Tickets[idx])
				msg.SetPriority(data.Priority)
				msgs[idx] = msg
				if !msg.Loggable() {
					ec.Add(fmt.Errorf("invalid/unlogable message at index %d, '%s'", idx, msg.String()))
				}
			}

			if !ec.Ok() {
				return ec.Resolve()
			}

			for idx, msg := range msgs {
				if ctx.Err() != nil {
					grip.Warning(message.Fields{
						"message":   "ticket creation aborted",
						"processed": idx - 1,
						"total":     len(msgs),
					})
					break
				}

				logger.Log(data.Priority, msg)

				grip.Info(message.Fields{
					"op":  "created jira issue",
					"idx": idx,
					"num": len(msgs),
					"str": msg.String(),
					"key": data.Tickets[idx].IssueKey,
				})
			}

			grip.Info(message.Fields{
				"op":  "bulk crate tickets",
				"num": len(data.Tickets),
				"ok":  ec.Ok(),
			})
			return nil
		})
}
