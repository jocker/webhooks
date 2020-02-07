package common

import (
	"github.com/sirupsen/logrus"
	"webhooks/common/data"
)

var Logger *logrus.Logger = logrus.New()

type MasterSyncRequestData struct {
	SlaveRangeStart int             `json:"slave_range_start"`
	SlaveRangeEnd   int             `json:"slave_range_end"`
	MasterIds       []data.ObjectID `json:"master_ids"`
}
