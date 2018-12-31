package units

import (
	"context"
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

type jiraBulkCreateJob struct {
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const jiraBulkCreateJobName = "jira-bulk-create"

func init() {
	registry.AddJobType(jiraBulkCreateJobName, func() amboy.Job { return jiraBulkCreateFactory() })
}
func jiraBulkCreateFactory() *jiraBulkCreateJob {
	j := &jiraBulkCreateJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    jiraBulkCreateJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewBulkCreateJiraTicketJob(path string) amboy.Job {
	j := jiraBulkCreateFactory()
	j.Path = path
	j.SetID(fmt.Sprintf("%s.%d.%s", jiraBulkCreateJobName, job.GetNumber(), j.Path))
	return j
}

func (j *jiraBulkCreateJob) Run(ctx context.Context) {
	data := struct {
		Priority level.Priority      `bson:"priority" json:"priority" yaml:"priority"`
		Tickets  []message.JiraIssue `bson:"tickets" json:"tickets" yaml:"tickets"`
	}{}

	err := util.UnmarshalFile(j.Path, &data)
	if err != nil {
		j.AddError(err)
		return
	}

	if !level.IsValidPriority(data.Priority) {
		data.Priority = level.Info
	}

	env := sardis.GetEnvironment()
	jira := env.JiraIssue()

	msgs := make([]message.Composer, len(data.Tickets))
	for idx := range data.Tickets {
		msg := message.MakeJiraMessage(&data.Tickets[idx])
		j.AddError(msg.SetPriority(data.Priority))
		msgs[idx] = msg
		if !msg.Loggable() {
			j.AddError(errors.Errorf("invalid/unlogable message at index %d, '%s'", idx, msg.String()))
		}
	}

	if j.HasErrors() {
		return
	}

	for idx, msg := range msgs {
		if ctx.Err() != nil {
			grip.Warning(message.Fields{
				"message":   "ticket creation aborted",
				"processed": idx - 1,
				"total":     len(msgs),
				"id":        j.ID(),
			})
		}

		jira.Log(data.Priority, msg)

		grip.Info(message.Fields{
			"op":  "created jira issue",
			"idx": idx,
			"num": len(msgs),
			"str": msg.String(),
			"key": data.Tickets[idx].IssueKey,
			"id":  j.ID(),
		})
	}

	grip.Info(message.Fields{
		"op":  "bulk crate tickets",
		"id":  j.ID(),
		"num": len(data.Tickets),
	})
}
