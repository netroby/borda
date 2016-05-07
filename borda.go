package borda

import (
	"time"
)

// Measurement represents a measurement at a point in time. It maps to a "point"
// in InfluxDB.
type Measurement struct {
	// Key is the name of the measurement (e.g. cpu_usage).
	Key string `json:"key"`

	// Timestamp records the time of the measurement.
	Timestamp time.Time `json:"timestamp,omitempty"`

	// IndexedDimensions capture metadata about the measurement. IndexedDimensions
	// are best for bounded values with relatively small cardinality. They map to
	// "tags" in InfluxDB.
	//
	// Example: {"host": "myhost.mydomain.com", "total_cpus": "2"}
	IndexedDimensions map[string]string `json:"indexed_dimensions,omitempty"`

	// UnindexedDimensions capture metadata about the measurement.  They map to
	// "fields" in InfluxDB. Unlikely IndexedDimensions, it's okay to include
	// high cardinality values here, but queries on UnindexedDimensions will be
	// slow.
	//
	// Example: {"requestid": "18af517b-004f-486c-9978-6cf60be7f1e9",
	//           "ipv6": "2001:0db8:0a0b:12f0:0000:0000:0000:0001"}
	UnindexedDimensions map[string]interface{} `json:"unindexed_dimensions,omitempty"`

	// Gauges represent a snapshot of a constantly shifting measurement at a point
	// in time.
	//
	// Example: {"idle": 10.1, "system": 53.3, "user": 36.6}
	Gauges map[string]float64 `json:"gauges,omitempty"`

	// Counts is a map of counts associated with the measurement, keyed to their
	// names.
	//
	// Example: {"swapouts": 512343, "swapins": 64534}
	Counts map[string]int64 `json:"counts,omitempty"`
}

// Collector collects Measurements.
type Collector struct {
}
