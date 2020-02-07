package common

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"webhooks/common/data"
)

func TestReadWebHookObject(t *testing.T) {
	dummy := map[string]interface{}{
		"a": "b",
		"b": map[string]interface{}{
			"d": "e",
		},
		"c": []string{"a", "b", "c"},
		"d": []int{1, 2, 4},
	}

	keys := []string{"a", "b", "c", "d"}

	now := time.Now()
	webhook := makeObj(t, dummy)
	xx := make(map[string]interface{})

	err := json.Unmarshal(webhook.JsonData, &xx)
	assert.NoError(t, err)
	for _, k := range keys {
		_, ok := xx[k]
		assert.True(t, ok, "expected to have key %s", k)
	}

	assert.False(t, webhook.ID.IsZero(), "should have a valid id")
	assert.True(t, webhook.ID.Timestamp().Before(now), "should have a valid timestamp")

	hash := webhook.ID.Hash()

	for _, k := range keys {
		v := dummy[k]
		delete(dummy, k)
		dummy[k] = v
		other := makeObj(t, dummy)
		assert.Equal(t, hash, other.ID.Hash(), "unsorted keys")
	}

	payload, _ := json.Marshal(dummy)
	a, err := ReadWebHookObject(bytes.NewReader(payload[:len(payload)-10]))
	assert.Nil(t, a)
	assert.NotNil(t, err)
}

func makeObj(t *testing.T, m map[string]interface{}) *data.WebHookObject {
	payload, _ := json.Marshal(m)
	webhook, err := ReadWebHookObject(bytes.NewReader(payload))
	assert.NoError(t, err)
	assert.NotNil(t, webhook)

	return webhook
}
