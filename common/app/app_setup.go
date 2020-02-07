package app

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"net/http"
	"os"
	"time"
	"webhooks/common"
	"webhooks/common/storage"
)

type StorageType int

const (
	_ = iota
	StorageTypeS3
	StorageTypeDynamoDb
)

func AppInitStrict(storageType StorageType) *App {
	a, err := AppInit(storageType)
	if err != nil {
		panic(err)
	}
	return a
}

// tries to init anything we need in the lambdas, returns an error if something goes wrong
func AppInit(storageType StorageType) (*App, error) {
	var awsCredentials *credentials.Credentials
	credsPath := os.Getenv("AWS_CREDENTIALS")
	if credsPath != "" {
		awsCredentials = credentials.NewSharedCredentials(credsPath, "default")
	}
	awsRegion := os.Getenv("REGION")
	if awsRegion == "" {
		awsRegion = "eu-central-1"
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(awsRegion),
		Credentials: awsCredentials,
	})

	if err != nil {
		panic(err)
	}

	var store storage.Store

	switch storageType {
	case StorageTypeS3:
		store = storage.NewDynamoDbStore(sess, os.Getenv("S3_BUCKET"))
	case StorageTypeDynamoDb:
		store = storage.NewDynamoDbStore(sess, os.Getenv("DYNAMO_TABLE"))
	default:
		return nil, fmt.Errorf("unknown storage type %v", storageType)
	}

	dataCollector := common.NewObjectBuffer(store, 100, time.Minute)

	return &App{
		Session:   sess,
		Store:     store,
		Collector: dataCollector,
	}, nil
}

// struct containing all required stuff needed by both master/slave lambdas
type App struct {
	Session   *session.Session
	Store     storage.Store
	Collector *common.ObjectBuffer
}

// receives post requests containing json objects
// writes the json data in storage
//this is common for slave/master - only the storage is different for them (slave -> s3, master -> dynamoDb)
func (app *App) CreateWebHookHttpHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		obj, err := common.ReadWebHookObject(request.Body)
		if err != nil {
			common.Logger.WithError(err).Error("error while reading webhook object")
			writer.WriteHeader(http.StatusInternalServerError)
		} else {
			app.Collector.Add(obj)
			writer.WriteHeader(http.StatusOK)
		}
	}
}
