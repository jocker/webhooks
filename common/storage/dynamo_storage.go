package storage

import (
	"context"
	"fmt"
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
	panic("todo")
}

func (s dbStorage) Objects(ctx context.Context, objectIds []data.ObjectID) (<-chan *data.WebHookObject, <-chan error) {
	panic("todo")
}
