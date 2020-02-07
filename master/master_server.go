package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apex/gateway"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"log"
	"net/http"
	"time"
	"webhooks/common"
	"webhooks/common/app"
	"webhooks/common/storage"
)

var App *app.App

func init() {
	App = app.AppInitStrict(app.StorageTypeDynamoDb)
}

func main() {
	http.HandleFunc("/webhook", App.CreateWebHookHttpHandler())
	http.HandleFunc("/trigger_sync", performSyncHandler)
	log.Fatal(gateway.ListenAndServe(":3000", nil))
}

func performSyncHandler(w http.ResponseWriter, r *http.Request) {

	err := performSync(r.Context())
	if err != nil {
		common.Logger.WithError(err).Error("sync failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)

}

func performSync(ctx context.Context) error {

	now := time.Now()

	existingIds, err := storage.LoadStorageKeysSync(ctx, App.Store, now.Add(time.Minute*-5), now.Add(time.Minute*-1))

	if err != nil {
		return err
	}

	syncReqData := &common.MasterSyncRequestData{
		SlaveRangeStart: int(now.Add(time.Minute * -5).Unix()),
		SlaveRangeEnd:   int(now.Add(time.Minute * -1).Unix()),
		MasterIds:       existingIds,
	}

	jsonData, err := json.Marshal(syncReqData)
	if err != nil {
		return err
	}

	input := &lambda.InvokeInput{
		FunctionName:   aws.String("slave-diff"),
		InvocationType: aws.String("RequestResponse"),
		LogType:        aws.String("Tail"),
		Payload:        jsonData,
	}

	svc := lambda.New(App.Session)

	result, err := svc.Invoke(input)
	if err != nil {
		return err
	}

	if *result.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %v", *result.StatusCode)
	}

	panic("TODO")

}
