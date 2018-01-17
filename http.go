package borda

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
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
	SampleRate           float64
	receivedMeasurements int64
}

// ServeHTTP implements the http.Handler interface and supports publishing measurements via HTTP.
func (h *Handler) Measurements(resp http.ResponseWriter, req *http.Request) {
	if h.SampleRate == 0 {
		h.SampleRate = 1
	}

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

	defer req.Body.Close()
	if rand.Float64() >= h.SampleRate {
		io.Copy(ioutil.Discard, req.Body)
	} else {
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
	}

	resp.WriteHeader(http.StatusCreated)
}

func (h *Handler) Ping(resp http.ResponseWriter, req *http.Request) {
	// this is just a ping, ignore the body and always return a 202
	resp.WriteHeader(http.StatusAccepted)
	return
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

// ForceCDN ensures that Get requests came through a CDN
func ForceCDN(h http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.Header.Get("Cf-Connecting-Ip") == "" && req.Header.Get("Cloudfront-Forwarded-Proto") == "" {
			// This didn't come through a CDN, redirect to CloudFlare
			req.URL.Host = "borda.lantern.io"
			http.Redirect(resp, req, req.URL.String(), http.StatusMovedPermanently)
			return
		}
		h.ServeHTTP(resp, req)
	})
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}
