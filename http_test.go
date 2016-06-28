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
		Name: "combined",
		Ts:   time.Now(),
		Values: map[string]float64{
			"field_float": 2.1,
		},
		Dimensions: map[string]interface{}{
			"dim_string":   "a",
			"dim_int":      1,
			"field_int":    2,
			"field_bool":   true,
			"field_string": "stringy",
		},
	}
	missingName = &Measurement{
		Ts:         time.Now(),
		Values:     good.Values,
		Dimensions: good.Dimensions,
	}
	missingTS = &Measurement{
		Name:       "mymeasure",
		Values:     good.Values,
		Dimensions: good.Dimensions,
	}
	missingValues = &Measurement{
		Name:       "mymeasure",
		Ts:         time.Now(),
		Dimensions: good.Dimensions,
	}
	emptyValues = &Measurement{
		Name:       "mymeasure",
		Ts:         time.Now(),
		Values:     map[string]float64{},
		Dimensions: good.Dimensions,
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

	s := func(m *Measurement) {
		done.Set(true)
	}
	h := &Handler{s}
	go http.Serve(hl, h)

	resp, _ := httpRequest(httpAddr, badContentType, []*Measurement{good})
	assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{missingName})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{missingTS})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{missingValues})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{emptyValues})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp, _ = httpRequest(httpAddr, goodContentType, []*Measurement{good})
	if !assert.Equal(t, http.StatusCreated, resp.StatusCode) {
		return
	}

	isDone, ok := done.Get(100 * time.Millisecond)
	if assert.True(t, ok) {
		assert.True(t, isDone.(bool))
	}
}

func httpRequest(addr string, contentType string, measurements []*Measurement) (*http.Response, error) {
	client := &http.Client{}
	b := new(bytes.Buffer)
	if measurements == nil {
		b.Write([]byte("Not valid JSON"))
	} else {
		err := json.NewEncoder(b).Encode(measurements)
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

func validateMeasurement(t *testing.T, allFloats bool, m *Measurement) {
	assert.Equal(t, "combined", m.Name, "Incorrect measurement key")
	assert.NotNil(t, point.Time(), "Missing timestamp")
	assert.Equal(t, map[string]interface{}{
		// Original dimensions, all are strings
		"dim_string": "a",
		"dim_int":    "1",
	}, point.Tags(), "Incorrect tags")

	var dimI interface{} = int64(1)
	var i interface{} = int64(2)
	if allFloats {
		dimI = float64(1)
		i = float64(2)
	}

	assert.Equal(t, map[string]interface{}{
		// Original fields
		"field_int":    i,
		"field_float":  float64(2.1),
		"field_bool":   true,
		"field_string": "stringy",

		// Synthetic fields for dimensions
		"_dim_string": "a",
		"_dim_int":    dimI,
	}, point.Fields(), "Incorrect fields")
}
