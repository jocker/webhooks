package storage

import (
	"context"
	"time"
	"webhooks/common/data"
)

type s3Storage struct {
}

func (s s3Storage) Put(ctx context.Context, data []*data.WebHookObject) error {
	panic("todo")
}

func (s s3Storage) Keys(ctx context.Context, fromTime, toTime time.Time) (<-chan data.ObjectID, <-chan error) {
	panic("todo")
}

func (s s3Storage) Objects(ctx context.Context, objectIds []data.ObjectID) (<-chan *data.WebHookObject, <-chan error) {
	panic("todo")
}

var _ Store = s3Storage{}
