package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getlantern/eventual"
	"github.com/stretchr/testify/assert"
)

func TestBordaClient(t *testing.T) {
	submitted := eventual.NewValue()
	ts := newMockServer(submitted)
	defer ts.Close()
	bordaURL = ts.URL

	bc := NewClient(
		&Options{
			BatchInterval: 100 * time.Millisecond,
		})
	assert.NotNil(t, bc)
	submit := bc.ReducingSubmitter("errors", 5)

	numUniqueErrors := 3
	countOfEachError := 10
	for i := 0; i < numUniqueErrors*countOfEachError; i++ {
		values := map[string]Val{
			"error_count": Float(1),
		}
		dims := map[string]interface{}{
			"ca": (i % numUniqueErrors) + 1,
			"cb": true,
		}
		submit(values, dims)
		_, sent := submitted.Get(0)
		assert.False(t, sent, "Shouldn't have sent the measurements yet")
	}

	time.Sleep(bc.options.BatchInterval * 3)
	_ms, sent := submitted.Get(0)
	if assert.True(t, sent, "Should have sent the measurements") {
		ms := _ms.([]map[string]interface{})
		if assert.Len(t, ms, numUniqueErrors, "Wrong number of measurements sent") {
			for i := 0; i < numUniqueErrors; i++ {
				m := ms[i]
				assert.EqualValues(t, countOfEachError, m["values"].(map[string]interface{})["error_count"])
				dims := m["dimensions"].(map[string]interface{})
				ca := dims["ca"].(float64)
				assert.True(t, 1 <= ca)
				assert.True(t, ca <= 3)
				assert.EqualValues(t, dims["cb"], true)
			}
		}
	}

	// Send another measurement and make sure that gets through too
	values := map[string]Val{
		"success_count": Float(1),
		"an_average":    Avg(2),
	}
	dims := map[string]interface{}{
		"cc": "c",
	}
	submit(values, dims)
	bc.Flush()
	_ms, _ = submitted.Get(0)
	if assert.Len(t, _ms, 1) {
		ms := _ms.([]map[string]interface{})
		assert.EqualValues(t, 1, ms[0]["values"].(map[string]interface{})["success_count"])
		assert.EqualValues(t, 2, ms[0]["values"].(map[string]interface{})["an_average"])
	}
}

func newMockServer(submitted eventual.Value) *httptest.Server {
	numberOfSuccesses := int32(0)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httputil.DumpRequest(r, true)
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Errorf("Error reading request: %v", err)
		} else {
			log.Tracef("Mock server received request: %v", string(dump))
		}

		decoder := json.NewDecoder(r.Body)
		var ms []map[string]interface{}
		err = decoder.Decode(&ms)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Error decoding JSON request: %v", err)
		} else {
			if atomic.AddInt32(&numberOfSuccesses, 1) == 1 {
				w.WriteHeader(500)
				fmt.Fprintf(w, "Failing on first success: %v", err)
			}
			w.WriteHeader(201)
			submitted.Set(ms)
		}
	}))

	return ts
}
