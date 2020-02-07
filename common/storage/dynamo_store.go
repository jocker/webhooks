package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"time"
	"webhooks/common/data"
)

const (
	dbColumnObjectId      = "object_id"
	dbColumnDate          = "date"
	dbColumnPayloadPrefix = "payload"
	dbDateFormat          = "2006-01-02"
)

var zeroTime time.Time

// implements Store for dynamodb
func NewDynamoDbStore(awsSession *session.Session, tableName string) Store {
	return dbStorage{
		db:        dynamodb.New(awsSession),
		tableName: tableName,
	}
}

type dbStorage struct {
	db        *dynamodb.DynamoDB
	tableName string
}

func (s dbStorage) Put(ctx context.Context, data []*data.WebHookObject) error {

	encoder := dynamodbattribute.NewEncoder()

	toWrite := make([]*dynamodb.WriteRequest, len(data))

	for idx, item := range data {
		jsonData := make(map[string]interface{})
		if err := item.DataTo(&jsonData); err != nil {
			return err
		}

		rawData := map[string]interface{}{
			dbColumnObjectId: item.ID.Hex(),
			dbColumnDate:     item.ID.Timestamp().UTC().Format(dbDateFormat),
		}

		for k, v := range jsonData {
			rawData[fmt.Sprintf("%s.%s", dbColumnPayloadPrefix, k)] = v
		}

		dbData := make(map[string]*dynamodb.AttributeValue, len(rawData))

		for k, v := range rawData {
			attrs, err := encoder.Encode(v)
			if err != nil {
				return err
			}
			dbData[k] = attrs
		}

		toWrite[idx] = &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: dbData,
			},
		}

	}

	params := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			s.tableName: toWrite,
		},
	}

	_, err := s.db.BatchWriteItemWithContext(ctx, params)
	return err

}

func (s dbStorage) Keys(ctx context.Context, fromTime, toTime time.Time) (<-chan data.ObjectID, <-chan error) {

	resChan := make(chan data.ObjectID)
	errChan := make(chan error, 1)

	go func() {

		defer close(resChan)
		defer close(errChan)

		decoder := dynamodbattribute.NewDecoder()

		dateIt := newDayIterator(fromTime, toTime)
		for {
			rangeStart, rangeEnd, ok := dateIt.Next()
			if !ok {
				return
			}

			objIdAttr := dbColumnObjectId
			//XXX is this the best api amazon can write?
			var queryInput = &dynamodb.QueryInput{
				TableName:       aws.String(s.tableName),
				AttributesToGet: []*string{&objIdAttr},
				KeyConditions: map[string]*dynamodb.Condition{
					dbColumnDate: {
						ComparisonOperator: aws.String("EQ"),
						AttributeValueList: []*dynamodb.AttributeValue{
							{
								S: aws.String(fromTime.Format(dbDateFormat)),
							},
						},
					},
					dbColumnObjectId: {
						ComparisonOperator: aws.String("BETWEEN"),
						AttributeValueList: []*dynamodb.AttributeValue{
							{
								S: aws.String(data.NewObjectIdFromTimestamp(rangeStart, 0).Hex()),
							},
							{
								S: aws.String(data.NewObjectIdFromTimestamp(rangeEnd, 0).Hex()),
							},
						},
					},
				},
			}

			var resp, err = s.db.QueryWithContext(ctx, queryInput)
			if err != nil {
				errChan <- err
				return
			}

			for _, item := range resp.Items {
				idHex := ""
				if err = decoder.Decode(item[dbColumnObjectId], &idHex); err != nil {
					errChan <- err
					return
				}
				if objId, err := data.NewObjectIdFromHex(idHex); err != nil {
					errChan <- err
					return
				} else {
					select {
					case resChan <- objId:
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					}
				}

			}

		}
	}()

	return resChan, errChan
}

func (s dbStorage) Objects(ctx context.Context, objectIds []data.ObjectID) (<-chan *data.WebHookObject, <-chan error) {
	// not needed
	errChan := make(chan error, 1)

	errChan <- errors.New("not implemented")
	close(errChan)
	return make(chan *data.WebHookObject, 0), errChan
}

// iterates over all day ranges between start and end time
func newDayIterator(start, end time.Time) *dayIterator {
	return &dayIterator{
		start: start.UTC(),
		end:   end.UTC(),
	}
}

type dayIterator struct {
	start time.Time
	end   time.Time
}

// returns the start, end times for the current day interval
func (it *dayIterator) Next() (time.Time, time.Time, bool) {

	start := it.start
	end := it.end

	if !start.Before(end) {
		return zeroTime, zeroTime, false
	}
	t := it.start
	nextStart := time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, t.Location())
	if nextStart.Before(end) {
		nextStart = end
	}

	it.start = nextStart
	return start, end, true
}
