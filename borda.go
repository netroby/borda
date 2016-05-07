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

	// Dimensions capture metadata about the measurement. Dimensions are indexed
	// and so are best for bounded values with relatively small cardinality. They
	// map to "tags" in InfluxDB.
	//
	// Example: { "host": "myhost.mydomain.com", "total_cpus": "2" }
	Dimensions map[string]string `json:"dimensions,omitempty"`

	// Values capture values and metadata about the measurement.  They map to
	// "fields" in InfluxDB. Unlikely Dimensions, it's okay to include
	// high cardinality values here, but queries on Values will be slower.
	//
	// Example: { "requestid": "18af517b-004f-486c-9978-6cf60be7f1e9",
	//            "ipv6": "2001:0db8:0a0b:12f0:0000:0000:0000:0001",
	//            "cpu_idle": 10.1,
	//            "cpu_system": 53.3,
	//            "cpu_user": 36.6 }
	Values map[string]interface{} `json:"unindexed_dimensions,omitempty"`
}

// Collector collects Measurements.
type Collector struct {
}
