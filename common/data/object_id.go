package data

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ZeroObjectID ObjectID

// a variation of mongo's objectId (8 bytes instead of 12) https://github.com/mongodb/mongo-go-driver/blob/master/bson/primitive/objectid.go
type ObjectID [8]byte

func NewObjectIdFromHex(str string) (ObjectID, error) {
	if len(str) != 16 {
		return ZeroObjectID, fmt.Errorf("invalid hex %s", str)
	}
	data, err := hex.DecodeString(str)
	if err != nil {
		return ZeroObjectID, err
	}

	var objId ObjectID
	copy(objId[:], data[:8])

	return objId, nil
}

func NewObjectIdFromTimestamp(timestamp time.Time, hash uint32) ObjectID {
	var b [8]byte
	binary.BigEndian.PutUint32(b[0:4], uint32(timestamp.Unix()))
	binary.BigEndian.PutUint32(b[4:], hash)

	return b
}

func (id ObjectID) MarshalJSON() ([]byte, error) {
	return json.Marshal(id.Hex())
}

func (id *ObjectID) UnmarshalJSON(b []byte) error {
	if len(b) == 8 {
		copy(id[:], b)
		return nil
	} else {
		var res interface{}
		err := json.Unmarshal(b, &res)
		if err != nil {
			return err
		}
		str, ok := res.(string)
		if !ok {
			return errors.New("unexpected input")
		}
		if len(str) != 16 {
			return fmt.Errorf("cannot unmarshal into an ObjectID, the length must be 8 but it is %d", len(str))
		}
		clone, err := NewObjectIdFromHex(str)
		if err != nil {
			return err
		}
		*id = clone
	}

	return nil
}

func (id ObjectID) Timestamp() time.Time {
	unixSecs := binary.BigEndian.Uint32(id[0:4])
	return time.Unix(int64(unixSecs), 0).UTC()
}

func (id ObjectID) Hash() uint32 {
	return binary.BigEndian.Uint32(id[4:])
}

func (id ObjectID) Hex() string {
	return hex.EncodeToString(id[:])
}

func (id ObjectID) String() string {
	return fmt.Sprintf("ObjectID(%q)", id.Hex())
}

func (id ObjectID) IsZero() bool {
	return bytes.Equal(id[:], ZeroObjectID[:])
}
