package borda

import (
	"time"

	"github.com/getlantern/golog"
)

var (
	log = golog.LoggerFor("borda")
)

// Measurement represents a measurement at a point in time.
type Measurement struct {
	// Name is the name of the measurement (e.g. cpu_usage).
	Name string `json:"name"`

	// Ts records the time of the measurement.
	Ts time.Time `json:"ts,omitempty"`

	// Values contains numeric values of the measurement.
	//
	// Example: { "num_errors": 67 }
	Values map[string]float64 `json:"values,omitempty"`

	// Dimensions captures key/value pairs which characterize the measurement.
	//
	// Example: { "requestid": "18af517b-004f-486c-9978-6cf60be7f1e9",
	//            "ipv6": "2001:0db8:0a0b:12f0:0000:0000:0000:0001",
	//            "host": "myhost.mydomain.com",
	//            "total_cpus": "2",
	//            "cpu_idle": 10.1,
	//            "cpu_system": 53.3,
	//            "cpu_user": 36.6,
	//            "connected_to_internet": true }
	Dimensions map[string]interface{} `json:"dimensions,omitempty"`
}

// SaveFunc is a function that saves a measurement
type SaveFunc func(m *Measurement) error
