package ghboop

import (
	"github.com/cbrgm/githubevents/githubevents"
	"github.com/google/go-github/v50/github"
)

func Beep() {
	handler := githubevents.New("foo")
	handler.OnWorkflowRunEventCompleted(func(deliveryID string, eventName string, event *github.WorkflowRunEvent) error { return nil })
	handler.OnIssuesEventAssigned(func(deliveryID string, eventName string, event *github.IssuesEvent) error { return nil })
	handler.OnIssuesEventOpened(func(deliveryID string, eventName string, event *github.IssuesEvent) error { return nil })
	handler.OnIssueCommentCreated(func(deliveryID string, eventName string, event *github.IssueCommentEvent) error { return nil })
}
