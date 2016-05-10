package borda

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/getlantern/eventual"
	"github.com/influxdata/influxdb/client/v2"
	"github.com/stretchr/testify/assert"
)

var (
	goodContentType = ContentTypeJSON
	badContentType  = "somethingelse"
	good            = &Measurement{
		Name: "mymeasure",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"dim_string":   "a",
			"dim_int":      1,
			"field_int":    2,
			"field_float":  2.1,
			"field_bool":   true,
			"field_string": "stringy",
		},
	}
	missingName = &Measurement{
		Ts:     time.Now(),
		Fields: good.Fields,
	}
	missingTS = &Measurement{
		Name:   "mymeasure",
		Fields: good.Fields,
	}
	missingFields = &Measurement{
		Name: "mymeasure",
		Ts:   time.Now(),
	}
	emptyFields = &Measurement{
		Name:   "mymeasure",
		Ts:     time.Now(),
		Fields: map[string]interface{}{},
	}
)

func TestHTTPRoundTrip(t *testing.T) {
	hl, err := net.Listen("tcp", "localhost:0")
	if !assert.NoError(t, err, "Unable to listen HTTP") {
		return
	}
	httpAddr := hl.Addr().String()

	done := eventual.NewValue()
	write := func(batch client.BatchPoints) error {
		validateBatch(t, true, batch)
		done.Set(true)
		return nil
	}

	c := NewCollector(&Options{
		Dimensions:      []string{"dim_string", "dim_int"},
		WriteToDatabase: write,
		DBName:          dbName,
		BatchSize:       1,
		MaxBatchWindow:  24 * time.Hour,
		MaxRetries:      5,
		RetryInterval:   5 * time.Millisecond,
	})
	go http.Serve(hl, c)

	resp, _ := request(httpAddr, badContentType, good)
	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, missingName)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, missingTS)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, missingFields)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, emptyFields)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = request(httpAddr, goodContentType, good)
	if !assert.Equal(t, http.StatusCreated, resp.StatusCode) {
		return
	}

	isDone, ok := done.Get(100 * time.Millisecond)
	if assert.True(t, ok) {
		assert.True(t, isDone.(bool))
	}
}

func request(addr string, contentType string, m *Measurement) (*http.Response, error) {
	client := &http.Client{}
	b := new(bytes.Buffer)
	if m == nil {
		b.Write([]byte("Not valid JSON"))
	} else {
		err := json.NewEncoder(b).Encode(m)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/measurements", b)
	if err != nil {
		return nil, err
	}
	req.Header.Set(ContentType, contentType)
	return client.Do(req)
}
