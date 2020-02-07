package common

import (
	"bytes"
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"time"
	"webhooks/common/data"
)

var (
	// known json delimiters - used for validating the json format
	DelimiterObjectStart *JsonDelimiter
	DelimiterObjectEnd   *JsonDelimiter
	DelimiterArrayStart  *JsonDelimiter
	DelimiterArrayEnd    *JsonDelimiter

	allKnownDelimiters map[string]*JsonDelimiter

	malformedJsonError = errors.New("invalid json")
)

type JsonDelimiter struct {
	value            string
	isStartDelimiter bool
	opposite         *JsonDelimiter
}

func (delim JsonDelimiter) String() string {
	return delim.value
}

func init() {
	DelimiterObjectStart = &JsonDelimiter{
		value:            "{",
		isStartDelimiter: true,
	}

	DelimiterObjectEnd = &JsonDelimiter{
		value:            "}",
		isStartDelimiter: false,
	}

	DelimiterArrayStart = &JsonDelimiter{
		value:            "[",
		isStartDelimiter: true,
	}

	DelimiterArrayEnd = &JsonDelimiter{
		value:            "]",
		isStartDelimiter: false,
	}

	DelimiterObjectStart.opposite = DelimiterObjectEnd
	DelimiterObjectEnd.opposite = DelimiterObjectStart

	DelimiterArrayStart.opposite = DelimiterArrayEnd
	DelimiterArrayEnd.opposite = DelimiterArrayStart

	// used for faster lookup
	allKnownDelimiters = map[string]*JsonDelimiter{
		DelimiterObjectStart.value: DelimiterObjectStart,
		DelimiterObjectEnd.value:   DelimiterObjectEnd,
		DelimiterArrayStart.value:  DelimiterArrayStart,
		DelimiterArrayEnd.value:    DelimiterArrayEnd,
	}

}

// reads a json object, validates it's format, sorts its keys, calculates the crc32c hash of the result
func ReadWebHookObject(in io.Reader) (*data.WebHookObject, error) {

	receivedAt := time.Now()

	// we need to traverse the in buffer twice
	//	- once for generating an unique json signature (because its keys are not ordered)
	// 	- once for reading the whole json - which we need to store somewhere
	results := make(map[string]*bytes.Buffer)
	var allJsonBuf bytes.Buffer
	tee := io.TeeReader(in, &allJsonBuf)

	dec := json.NewDecoder(tee)

	t, err := dec.Token()
	if err != nil {
		return nil, nil
	}

	jsonDelim, ok := t.(json.Delim)
	if !ok {
		return nil, malformedJsonError
	}
	if jsonDelim.String() != DelimiterObjectStart.value {
		return nil, malformedJsonError
	}

	for {
		err = readJsonObjectPair(dec, results)
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
	}

	hash, err := digestPayload(results)
	if err != nil {
		return nil, err
	}

	return &data.WebHookObject{
		ID:       data.NewObjectIdFromTimestamp(receivedAt, hash),
		JsonData: allJsonBuf.Bytes(),
	}, nil

}

func readJsonObjectPair(dec *json.Decoder, dest map[string]*bytes.Buffer) (readError error) {

	delims := list.New() // keeps track of the json delimiter tokens
	var jsonValue *bytes.Buffer
	tkn, err := dec.Token()

	if err != nil {
		return err
	}
	var jsonKey string
	// json object keys must be strings
	if key, ok := tkn.(string); ok {
		jsonKey = key
	} else {
		err = malformedJsonError
		goto DONE
	}

	for {
		tkn, err = dec.Token()

		if err != nil {
			readError = err
			goto DONE
		}
		jsonDelim, ok := tkn.(json.Delim)
		if ok {
			knownDelim := findKnownJsonDelimiter(jsonDelim.String())
			if knownDelim == nil {
				readError = malformedJsonError
				goto DONE
			}
			if knownDelim.isStartDelimiter {
				delims.PushBack(knownDelim)
			} else {
				prev := delims.Back()
				if prev == nil {
					// an open delimiter doesn't have a close delim
					readError = malformedJsonError
					goto DONE
				}
				if v, ok := prev.Value.(*JsonDelimiter); ok && v.opposite == knownDelim {
					// close delimiter doesn't match with the one that's opened
					delims.Remove(prev)
				} else {
					readError = malformedJsonError
					goto DONE
				}
			}
		} else { //no delimiter. This should be a primitive json value (number, string, null, etc)
			var toWrite string
			// stringifying the json value
			if tkn == nil {
				// nothing to append
				toWrite = ""
			} else if str, ok := tkn.(string); ok {
				toWrite = str
			} else {
				str := fmt.Sprintf("%v", tkn)
				toWrite = str
			}
			if toWrite != "" {
				if jsonValue == nil {
					jsonValue = bytes.NewBuffer([]byte(toWrite))
				} else {
					jsonValue.Write([]byte(toWrite))
				}
			}
		}

		if delims.Len() == 0 {
			break
		}

	}

DONE:
	if readError != nil {
		return readError
	}
	if jsonKey == "" {
		return
	}
	dest[jsonKey] = jsonValue
	return nil
}

func findKnownJsonDelimiter(token string) *JsonDelimiter {
	if v, ok := allKnownDelimiters[token]; ok {
		return v
	}
	return nil
}

// generating a crc32c hash for the given json object - having keys sorted
func digestPayload(data map[string]*bytes.Buffer) (uint32, error) {
	keys := make([]string, len(data))
	i := 0
	for k := range data {
		keys[i] = k
		i += 1
	}

	sort.Strings(keys)
	tablePolynomial := crc32.MakeTable(uint32(crc32.Castagnoli))

	hash32 := crc32.New(tablePolynomial)

	for _, k := range keys {
		if _, err := hash32.Write([]byte(k)); err != nil {
			return 0, err
		}
		if _, err := hash32.Write(data[k].Bytes()); err != nil {
			return 0, err
		}
	}
	return hash32.Sum32(), nil
}
