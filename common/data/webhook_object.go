package data

import (
	"encoding/json"
	"errors"
	"time"
)

type WebHookObject struct {
	ID       ObjectID
	JsonData []byte // actual json byte array - note that this might be nil
}

func (this *WebHookObject) DataTo(pointer interface{}) error {
	if this.JsonData == nil {
		return errors.New("missing data")
	}
	return json.Unmarshal(this.JsonData, pointer)
}

func (this *WebHookObject) ToInterfaceMap() (map[string]interface{}, error) {
	x := map[string]interface{}{}
	err := this.DataTo(&x)
	return x, err
}

func (this WebHookObject) Timestamp() time.Time {
	return this.ID.Timestamp()
}

func (this WebHookObject) UnixTimestamp() int64 {
	return this.ID.Timestamp().Unix()
}
