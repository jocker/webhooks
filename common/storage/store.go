package storage

import (
	"context"
	"time"
	"webhooks/common/data"
)

type Store interface {
	Put(ctx context.Context, data []*data.WebHookObject) error

	Keys(ctx context.Context, fromTime, toTime time.Time) (<-chan data.ObjectID, <-chan error)

	Objects(ctx context.Context, ids []data.ObjectID) (<-chan *data.WebHookObject, <-chan error)
}

func LoadStorageKeysSync(ctx context.Context, store Store, rangeStart, rangeEnd time.Time) ([]data.ObjectID, error) {
	idsChan, errChan := store.Keys(ctx, rangeStart, rangeEnd)
	res := make([]data.ObjectID, 0)
	isDone := false
	for !isDone {
		select {
		case id, ok := <-idsChan:
			if ok {
				res = append(res, id)
			} else {
				isDone = true
			}
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

func LoadStorageObjectsSync(ctx context.Context, store Store, ids []data.ObjectID) ([]*data.WebHookObject, error) {
	idsChan, errChan := store.Objects(ctx, ids)
	res := make([]*data.WebHookObject, 0)
	isDone := false
	for !isDone {
		select {
		case id, ok := <-idsChan:
			if ok {
				res = append(res, id)
			} else {
				isDone = true
			}
		case err := <-errChan:
			if err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}
