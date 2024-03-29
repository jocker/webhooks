package data

import (
	"crypto/md5"
	"encoding/base64"
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

func (this WebHookObject) Md5() string {
	if this.JsonData == nil {
		return ""
	}
	h := md5.New()
	h.Write(this.JsonData)

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
