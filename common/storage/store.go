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
