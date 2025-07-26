package dblock

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ers"
)

type Configuration struct {
	URI        string
	Database   string
	Collection string
	InstanceID string
	Prefix     string
	TTL        time.Duration
}

type LockRecord struct {
	Name     string    `bson:"_id" json:"_id"`
	Active   bool      `bson:"in_use" json:"in_use"`
	ModTime  time.Time `bson:"mod_time" json:"mod_time"`
	ModCount int       `bson:"mod_count" json:"mod_count"`
	Owner    string    `bson:"owner" json:"owner"`
}

type LockService interface {
	GetLock(context.Context, string) Lock
	CheckLock(context.Context, string) (bool, error)
}

type Lock interface {
	Lock() fun.Operation
	Unlock()
}

type mdbLockService struct {
	conf   *Configuration
	client *mongo.Client
}

func NewLockService(_ *Configuration) (LockService, error) {
	return new(mdbLockService), nil

}

func (ls *mdbLockService) GetLock(_ context.Context, name string) Lock {
	lock := &mdbLockImpl{
		srv:    ls,
		waiter: &adt.Atomic[*fun.Operation]{},
	}
	lock.record.Name = name
	return lock
}

func (ls *mdbLockService) CheckLock(ctx context.Context, name string) (bool, error) {
	coll := ls.collection()
	num, err := coll.CountDocuments(ctx, bson.M{"_id": name})
	if err != nil {
		return false, err
	}
	return num == 1, nil
}

type mdbLockImpl struct {
	ctx    context.Context
	srv    *mdbLockService
	record LockRecord
	waiter *adt.Atomic[*fun.Operation]
}

func (l *mdbLockImpl) getNextQuery(now time.Time) bson.M {
	return bson.M{
		"_id": l.record.Name,
		"$or": []bson.M{
			{"in_use": false},
			{"in_use": true, "mod_time": bson.M{"$lte": now.Add(-l.srv.conf.TTL)}},
		},
		"mod_count": bson.M{"$gte": l.record.ModCount},
	}
}

func (ls *mdbLockService) collection() *mongo.Collection {
	return ls.client.Database(ls.conf.Database).Collection(ls.conf.Collection)
}

func (l *mdbLockImpl) updateLockView(ctx context.Context) error {
	coll := l.srv.collection()
	now := time.Now().UTC().Round(time.Millisecond)

	nextQuery := l.getNextQuery(now)
	res := coll.FindOne(ctx, nextQuery)
	if res.Err() == nil {
		if err := res.Decode(&l.record); err != nil {
			return err
		}
		l.record.ModCount++
		l.record.ModTime = now
	}
	return res.Err()
}

func (l *mdbLockImpl) Lock() fun.Operation {
	ctx, cancel := context.WithCancel(l.ctx)
	coll := l.srv.collection()
	err := l.updateLockView(ctx)

	var res *mongo.SingleResult
	for !ers.IsExpiredContext(err) {

		if err != nil {
			l.record.Active = true
			l.record.Owner = l.srv.conf.InstanceID

			res = coll.FindOneAndUpdate(ctx,
				l.getNextQuery(l.record.ModTime),
				l.record,
				options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
			)
			err = res.Err()
			if err == nil {
				if err = res.Decode(&l.record); err != nil {
					panic(err)
				}
			}
		}
		if err == nil {
			if err = res.Decode(&l.record); err != nil {
				panic(err)
			}
			if !l.record.Active {
				continue
			}
			if l.record.Owner != l.srv.conf.InstanceID {
				continue
			}
			break
		}

		// someone else owns the lock
		err = l.updateLockView(ctx)
	}

	signal := l.startBackgroundLockPing(ctx)

	var release func()
	waiter := fun.Operation(func(ctx context.Context) {
		defer release()
		defer cancel()
		select {
		case <-ctx.Done():
		case <-signal:
		}
	})

	release = func() {
		adt.CompareAndSwap[*fun.Operation](l.waiter, &waiter, nil)
	}

	for {
		if adt.CompareAndSwap[*fun.Operation](l.waiter, nil, &waiter) {
			return waiter
		}

		// if there's another wait in progress, we already
		// hold the lock, and someone else locally has locked
		// this lock; therefore we can hand ourselves the
		// existing lock.
		if nw := l.waiter.Get(); nw != nil {
			cancel()
			return *nw
		}
	}
}

func (l *mdbLockImpl) startBackgroundLockPing(ctx context.Context) chan struct{} {
	signal := make(chan struct{})
	go func() {
		defer close(signal)
		coll := l.srv.collection()

		ticker := time.NewTicker(l.srv.conf.TTL / 2)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now().UTC().Round(time.Millisecond)
				timeout := now.Add(-l.srv.conf.TTL)
				res, err := coll.UpdateOne(ctx,
					// query
					bson.M{
						"_id": l.record.Name,
						"$or": []bson.M{
							{
								"owner":     l.record.Owner,
								"mod_time":  bson.M{"$gt": timeout},
								"mod_count": bson.M{"$in": []int{l.record.ModCount, l.record.ModCount - 1}},
							},
							{"mod_time": bson.M{"$lte": timeout}},
						},
					},
					// update
					bson.M{
						"mod_count": bson.M{"$inc": 1},
						"mod_time":  now,
					},
				)
				if err != nil || res.ModifiedCount != 1 {
					return
				}
			}
		}
	}()
	return signal
}

func (l *mdbLockImpl) Unlock() {
	if waiter := l.waiter.Get(); waiter != nil {
		wf := *waiter
		wf.Wait()
		return
	}
}
