package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/apex/gateway"
	"io"
	"log"
	"net/http"
	"time"
	"webhooks/common"
	"webhooks/common/app"
	"webhooks/common/data"
	"webhooks/common/storage"
)

var App *app.App

func init() {
	App = app.AppInitStrict(app.StorageTypeDynamoDb)
}

func main() {
	http.HandleFunc("/webhook", App.CreateWebHookHttpHandler())
	http.HandleFunc("/master_sync", func(writer http.ResponseWriter, request *http.Request) {
		err := replyToSync(request.Context(), request.Body, writer, App.Store)
		if err != nil {
			common.Logger.WithError(err).Errorf("slave couldn't handle request")
			writer.WriteHeader(http.StatusInternalServerError)
		} else {
			writer.WriteHeader(http.StatusOK)
		}

	})

	log.Fatal(gateway.ListenAndServe(":3000", nil))
}

// takes an input stream containing a master diff request
//		with all objects which were not found in the request ids
// TODO this will years away from being good if we have to process 12k records - we will need to use streams.
// 	- No time for that now
//	- The idea is simple however - all the code needs to be context aware and it needs to process items as they come in (channels++++ goroutines+++)
// 	- context.cancel stops everything, an error in any of the processors stops everything
func replyToSync(ctx context.Context, in io.Reader, out io.Writer, store storage.Store) error {

	reqData := common.MasterSyncRequestData{}

	err := json.NewDecoder(in).Decode(reqData)
	if err != nil {
		return err
	}

	//TODO other checks can be done here - like checking if the ids are sorted

	// loading all slave ids for the given interval
	slaveIds, err := storage.LoadStorageKeysSync(
		ctx,
		store,
		time.Unix(int64(reqData.SlaveRangeStart), 0),
		time.Unix(int64(reqData.SlaveRangeEnd), 0),
	)

	if err != nil {
		return err
	}

	// check which ids are present in slave but not in master
	missingMasterIds := getDiffIds(reqData.MasterIds, slaveIds, time.Minute)

	// load the slave objects based on the ids we found above
	missingMasterObjects, err := storage.LoadStorageObjectsSync(ctx, store, missingMasterIds)

	if err != nil {
		return err
	}

	// replying back with the objects missing from master
	var buf bytes.Buffer
	for _, obj := range missingMasterObjects {
		buf.Reset()
		if err = writeObjects(&buf, obj); err != nil {
			return err
		}
		if _, err = out.Write(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// returns the records from slaveData which were not found in the master data
func getDiffIds(masterData []data.ObjectID, slaveData []data.ObjectID, maxTimeSpan time.Duration) []data.ObjectID {
	maxDiff := int64(maxTimeSpan.Seconds())
	masterIndex := 0
	results := make([]data.ObjectID, 0)

	for _, slaveId := range slaveData {
		// if we already read all the master data, it means all the records left in slaveData need to be appended to the result
		missingFromMaster := masterIndex == len(masterData)-1

		if !missingFromMaster {
			slaveCreatedAt := slaveId.Timestamp().Unix()
			minTs := slaveCreatedAt - maxDiff
			maxTs := slaveCreatedAt + maxDiff

			// reposition the master index
			// masterIndex should point to the first master entry having the biggest timestamp < slave entry timestamp
			for i := masterIndex; i >= 0; i-- {
				masterIndex = i
				if masterData[i].Timestamp().Unix() < minTs {
					break
				}
			}

		MASTER_LOOP:
			for i := masterIndex; i < len(masterData); i++ {
				masterEntry := masterData[i]

				if masterEntry.Timestamp().Unix() > maxTs {
					masterIndex = i
					missingFromMaster = true
					break MASTER_LOOP
				}

				if masterEntry.Hash() == slaveId.Hash() {
					missingFromMaster = false
					masterIndex = i
					break MASTER_LOOP
				}
			}
		}
		if missingFromMaster {
			results = append(results, slaveId)
		}
	}

	return results
}

// write a json entry to the buffer
// format is {id:objectId, data:original json data }
// note that data is this time a map[string]interface, not a binary array
func writeObjects(dest *bytes.Buffer, item *data.WebHookObject) error {

	dest.WriteString(common.DelimiterObjectStart.String())
	dest.WriteString(fmt.Sprintf("\"id\":\"%s\",", item.ID.Hex()))
	dest.WriteString("\",data\":")
	dest.Write(item.JsonData) // this is a binary array
	dest.WriteString(common.DelimiterObjectEnd.String())

	return nil
}
