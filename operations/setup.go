package operations

import (
	"os"

	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
)

const (
	NotifyTargetEnvName = "SARDIS_NOTIFY_TARGET"
	numWorkers          = 2
)

type configureFunc func() error

// TODO populate global session queue.

func configureAll() error {
	confs := []configureFunc{
		configureQueue,
		configureSender,
	}

	// run all the functions
	catcher := grip.NewCatcher()
	for _, setup := range confs {
		catcher.Add(setup())
	}

	return catcher.Resolve()
}

func configureQueue() error {
	q := queue.NewLocalLimitedSize(numWorkers, 1024)
	grip.Infof("configured local queue with %d workers", numWorkers)
	if err := sardis.SetQueue(q); err != nil {
		return errors.Wrap(err, "problem configuring queue")
	}

	return nil
}

func configureSender() error {
	sender, err := send.NewXMPP("sardis", os.Getenv(NotifyTargetEnvName), grip.GetSender().Level())
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}

	if err = sardis.SetSystemSender(sender); err != nil {
		return errors.Wrap(err, "problem setting the global sender")
	}

	return nil
}
