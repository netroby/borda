package borda

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	// ContentType is the key for the Content-Type header
	ContentType = "Content-Type"

	// ContentTypeJSON is the allowed content type
	ContentTypeJSON = "application/json"
)

// Handler is an http.Handler that reads Measurements from HTTP and saves them
// to the database.
type Handler struct {
	Save                 SaveFunc
	receivedMeasurements int64
}

// ServeHTTP implements the http.Handler interface and supports publishing measurements via HTTP.
func (h *Handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(resp, "Method %v not allowed\n", req.Method)
		return
	}

	contentType := req.Header.Get(ContentType)
	if contentType != ContentTypeJSON {
		resp.WriteHeader(http.StatusUnsupportedMediaType)
		fmt.Fprintf(resp, "Media type %v unsupported\n", contentType)
		return
	}

	dec := json.NewDecoder(req.Body)
	var measurements []*Measurement
	err := dec.Decode(&measurements)
	if err != nil {
		badRequest(resp, "Error decoding JSON: %v", err)
		return
	}

	if len(measurements) == 0 {
		badRequest(resp, "Please include at least 1 measurement", err)
		return
	}

	for _, m := range measurements {
		if m.Name == "" {
			badRequest(resp, "Missing name")
			return
		}

		if m.Ts.IsZero() {
			badRequest(resp, "Missing ts")
			return
		}

		if m.Values == nil || len(m.Values) == 0 {
			badRequest(resp, "Need at least one value")
			return
		}
	}

	atomic.AddInt64(&h.receivedMeasurements, int64(len(measurements)))
	log.Tracef("Received %d measurements", len(measurements))
	for _, m := range measurements {
		err := h.Save(m)
		if err != nil {
			log.Errorf("Error saving measurement, continuing: %v", err)
		}
	}

	resp.WriteHeader(http.StatusCreated)
}

func (h *Handler) Report() {
	start := time.Now()
	ticker := time.NewTicker(15 * time.Second)
	for range ticker.C {
		delta := time.Now().Sub(start)
		measurements := float64(atomic.SwapInt64(&h.receivedMeasurements, 0))
		tps := measurements / delta.Seconds()
		log.Debugf("Processed %d measurements at %d per second", int64(measurements), int(tps))
		start = time.Now()
	}
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}
