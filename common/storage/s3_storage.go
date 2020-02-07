package storage

import (
	"bytes"
	"container/list"
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"sync"
	"time"
	"webhooks/common/data"
)

func NewS3Store(awsSession *session.Session, bucket string) Store {
	return s3Storage{
		session:           awsSession,
		bucket:            bucket,
		listKeysBatchSize: 1000,
	}
}

type s3Storage struct {
	session           *session.Session
	bucket            string
	listKeysBatchSize int
}

func (s s3Storage) Put(ctx context.Context, data []*data.WebHookObject) error {
	objects := make([]s3manager.BatchUploadObject, len(data))

	for i := 0; i < len(data); i++ {
		payload := data[i]

		objects[i] = s3manager.BatchUploadObject{
			Object: &s3manager.UploadInput{
				ACL:         nil,
				Body:        bytes.NewReader(payload.JsonData),
				Bucket:      aws.String(s.bucket),
				Key:         aws.String(payload.ID.Hex()),
				ContentType: aws.String("application/json"),
				ContentMD5:  aws.String(payload.Md5()),
			},
		}
	}

	uploader := s3manager.NewUploader(s.session)

	return uploader.UploadWithIterator(ctx, &s3manager.UploadObjectsIterator{
		Objects: objects,
	})
}

func (s s3Storage) Keys(ctx context.Context, fromTime, toTime time.Time) (<-chan data.ObjectID, <-chan error) {

	resChan := make(chan data.ObjectID, s.listKeysBatchSize)
	errChan := make(chan error, 1)

	go func() {

		defer close(resChan)
		defer close(errChan)

		startObjectId := data.NewObjectIdFromTimestamp(fromTime, 0)

		awsS3 := s3.New(s.session)

		for !startObjectId.IsZero() {
			resp, err := awsS3.ListObjectsWithContext(ctx, &s3.ListObjectsInput{
				Bucket:  aws.String(s.bucket),
				Marker:  aws.String(startObjectId.Hex()),
				MaxKeys: aws.Int64(int64(s.listKeysBatchSize)),
			})

			if err != nil {
				errChan <- err
				return
			}

			if len(resp.Contents) == 0 {
				return
			}

			endIndex := len(resp.Contents) - 1

			// in case th page size == response size => we need to find the start key for the next request
			if endIndex == s.listKeysBatchSize-1 { // assuming pageSize >= 2
				endIndex -= 1
				for i := endIndex; i >= 0; i-- {
					//TODO handle bad data and errors
					currentItem, _ := data.NewObjectIdFromHex(*resp.Contents[i].Key)
					nextItem, _ := data.NewObjectIdFromHex(*resp.Contents[i+1].Key)

					startObjectId = nextItem

					if !currentItem.Timestamp().Equal(nextItem.Timestamp()) {
						break
					}
				}
			} else {
				startObjectId = data.ZeroObjectID
			}

			for i := 0; i <= endIndex; i++ {
				//TODO handle bad data and errors
				objId, _ := data.NewObjectIdFromHex(*resp.Contents[i].Key)

				if objId.Timestamp().After(toTime) {
					return
				}

				select {
				case <-ctx.Done():
					return
				case resChan <- objId:
					continue
				}
			}

		}

	}()

	return resChan, errChan
}

func (s s3Storage) Objects(ctx context.Context, objectIds []data.ObjectID) (<-chan *data.WebHookObject, <-chan error) {

	monitor := &downloadMonitor{
		List:    list.New(),
		ctx:     ctx,
		bucket:  s.bucket,
		session: s.session,
	}

	monitor.AddIds(objectIds)
	return monitor.Run()

}

type downloadItemState struct {
	obj    *data.WebHookObject
	isDone bool
}

// keeps track of the objects that were donwloaded
// ensures proper ordering of the result
type downloadMonitor struct {
	*list.List
	itemDoneMux sync.Mutex
	ctx         context.Context
	bucket      string
	session     *session.Session
}

// enqueue an item for download
func (m *downloadMonitor) Add(item *data.WebHookObject) {
	m.PushBack(&downloadItemState{
		obj:    item,
		isDone: false,
	})
}

// enqueue items for download by their id
func (m *downloadMonitor) AddIds(objectIds []data.ObjectID) {
	for i := 0; i < len(objectIds); i++ {
		m.Add(&data.WebHookObject{
			ID:       objectIds[i],
			JsonData: []byte{},
		})
	}
}

// we need to emit items in the same order they were requested
// item N can't be sent back unless all items before it were done and sent
func (m *downloadMonitor) setItemDownloaded(item *downloadItemState, jsonData []byte, outChan chan<- *data.WebHookObject) error {
	m.itemDoneMux.Lock()
	defer m.itemDoneMux.Unlock()

	front := m.Front()
	isFront := false
	for current := m.Front(); current != nil; current = current.Next() {
		stateObj := current.Value.(*downloadItemState)
		if stateObj == item {
			stateObj.obj.JsonData = jsonData
			stateObj.isDone = true
			if current == front {
				isFront = true
			}
			break
		}
	}

	if isFront {
		for current := m.Front(); current != nil; current = m.Front() {

			stateObj := current.Value.(*downloadItemState)
			if stateObj.isDone {
				select {
				case outChan <- stateObj.obj:
				//ok
				case <-m.ctx.Done():
					return m.ctx.Err()
				}
				m.Remove(current)
			} else {
				break
			}
		}
	}
	return nil
}

// starts the actual download
// resChan will emit all the objects that were downloaded in the same order they were requested
// errChan used for notifying about errors
func (m *downloadMonitor) Run() (<-chan *data.WebHookObject, chan error) {

	resChan := make(chan *data.WebHookObject)
	errChan := make(chan error, 1)

	go func() {

		defer close(resChan)
		defer close(errChan)

		objects := make([]s3manager.BatchDownloadObject, m.Len())
		i := 0
		for current := m.Front(); current != nil; current = current.Next() {
			stateObj := current.Value.(*downloadItemState)

			writer := aws.NewWriteAtBuffer([]byte{})

			objects[i] = s3manager.BatchDownloadObject{
				Object: &s3.GetObjectInput{
					Bucket: aws.String(m.bucket),
					Key:    aws.String(stateObj.obj.ID.Hex()),
				},
				Writer: writer,
				After: func() error {
					return m.setItemDownloaded(stateObj, writer.Bytes(), resChan)
				},
			}

			i += 1
		}

		downloader := s3manager.NewDownloader(m.session)

		err := downloader.DownloadWithIterator(m.ctx, &s3manager.DownloadObjectsIterator{
			Objects: objects,
		})

		if err != nil {
			errChan <- err
			return
		}

	}()

	return resChan, errChan
}

var _ Store = s3Storage{}
