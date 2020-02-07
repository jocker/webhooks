package storage

import (
	"context"
	"github.com/aws/aws-sdk-go/aws/session"
	"time"
	"webhooks/common/data"
)

func NewS3Store(awsSession *session.Session, bucket string) Store {
	return s3Storage{
		session: awsSession,
		bucket:  bucket,
	}
}

type s3Storage struct {
	session *session.Session
	bucket  string
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
