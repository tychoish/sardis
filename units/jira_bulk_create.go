package units

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/x/jira"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

func NewBulkCreateJiraTicketJob(path string) fun.Worker {
	return func(ctx context.Context) error {
		data := struct {
			Priority level.Priority `bson:"priority" json:"priority" yaml:"priority"`
			Tickets  []jira.Issue   `bson:"tickets" json:"tickets" yaml:"tickets"`
		}{}

		err := util.UnmarshalFile(path, &data)
		if err != nil {
			return err
		}

		data.Priority = level.Info

		conf := sardis.AppConfiguration(ctx)
		ctx = sardis.WithJiraIssue(ctx, conf)
		logger := sardis.JiraIssue(ctx)
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

		if ec.HasErrors() {
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
			"op":     "bulk crate tickets",
			"num":    len(data.Tickets),
			"errors": ec.HasErrors(),
		})
		return nil
	}
}
