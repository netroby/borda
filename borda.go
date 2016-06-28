package borda

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/getlantern/eventual"
	"github.com/golang/glog"
	"github.com/influxdata/influxdb/client/v2"
)

// Measurement represents a measurement at a point in time. It maps to a "point"
// in InfluxDB.
type Measurement struct {
	// Name is the name of the measurement (e.g. cpu_usage). It maps to the "key"
	// in "InfluxDB".
	Name string `json:"name"`

	// Ts records the time of the measurement.
	Ts time.Time `json:"ts,omitempty"`

	// Values contains numeric values of the measurement. These will be stored as
	// "fields" in InfluxDB.
	//
	// Example: { "num_errors": 67 }
	Values map[string]float64 `json:"values,omitempty"`

	// Dimensions captures key/value pairs which characterize the measurement.
	// Dimensions are stored as "tags" or "fields" in InfluxDB depending on which
	// dimensions have been configured as "IndexedDimensions" on the Collector.
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
