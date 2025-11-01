package postal

import (
	"errors"
	"fmt"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

type Schema struct {
	Version int
	Type    string
	Format  SerializationFormat
}

type SerializationFormat string

const (
	SerializationCodecStdlibJSON SerializationFormat = "stdlib-json"
	SerializationCodecBirchBSON  SerializationFormat = "birch-bson"
	SerializationCodecDriverBSON SerializationFormat = "driver-bson"
	SerializationCodecYAML       SerializationFormat = "yaml"
)

func (s Schema) Validate() error {
	ec := &erc.Collector{}
	ec.If(s.Version < 0, ers.New("schema version must not be negative"))
	ec.If(s.Type < "", ers.New("message type must be specified"))

	return ec.Resolve()
}

type LockInfo struct {
	Status       LockStatus
	LogicalClock uint64
	LastModified time.Time
	Owner        string
}

func (l *LockInfo) Update(status LockStatus) {
	fun.Invariant.IsTrue(l.LogicalClock+1 != 0, "cannot overflow the logical clock")
	fun.Invariant.IsTrue(l.LastModified.IsZero() || time.Since(l.LastModified) >= time.Millisecond,
		"lock time resolution requires at least 1ms between updates")
	fun.Invariant.Must(status.ValidateStateTransion(status), "cannot apply impossible state transition ")

	l.LogicalClock++
	l.LastModified = time.Now().UTC().Truncate(time.Millisecond)
	l.Status = status
}

type LockStatus string

const (
	LockStatusCreated        LockStatus = "created"
	LockStatusInProgress     LockStatus = "in-progress"
	LockStatusAvailable      LockStatus = "available"
	LockStatusComplete       LockStatus = "completed"
	LockStatusExpired        LockStatus = "expired"
	LockStatusErrorRetryable LockStatus = "error-retryable"
	LockStatusErrorFatal     LockStatus = "error-fatal"
)

func (l LockStatus) ValidateStateTransion(to LockStatus) error {
	err := to.Validate()

	switch {
	case l == "":
		return nil
	case l == "" && to == "":
		return nil
	case err != nil:
		return err
	case l == to:
		return nil
	case l == LockStatusComplete:
		return errors.New("complete locks should not be further modified")
	case l == LockStatusErrorFatal:
		return errors.New("fatal locks can never be retried")
	case l == LockStatusExpired:
		return errors.New("expired locks cannot revive")
	case l == LockStatusInProgress && to == LockStatusCreated:
		return errors.New("cannot transition to created")
	default:
		return nil
	}
}

func (l LockStatus) Validate() error {
	switch l {
	case "":
		return errors.New("lock status must be defined")
	case LockStatusCreated, LockStatusInProgress, LockStatusAvailable:
		return nil
	case LockStatusComplete, LockStatusExpired:
		return nil
	case LockStatusErrorFatal, LockStatusErrorRetryable:
		return nil
	default:
		return fmt.Errorf("lock status %q is invalid", l)
	}
}

func (l LockStatus) String() string {
	err := l.Validate()
	switch {
	case l == "":
		return string(LockStatusCreated)
	case err != nil:
		return fmt.Sprintf("INVALID<%s>", err.Error())
	default:
		return string(l)
	}
}
