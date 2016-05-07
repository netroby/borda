package borda

import (
	"time"
)

// Measurement represents a measurement at a point in time. It maps to a "point"
// in InfluxDB. borda collects Measurements and saves them to the database in
// batches. During this process, borda may downsample Measurements by grouping
// them by their Dimensions (including IndexedDimensions) and then using the
// most recent Gauges and Counts and aggregating all Increments.
type Measurement struct {
	// Key is the name of the measurement (e.g. cpu_usage).
	Key string

	// Timestamp records the time of the measurement.
	Timestamp time.Time

	// IndexedDimensions capture metadata about the measurement. IndexedDimensions
	// are best for bounded values with relatively small cardinality. They map to
	// "tags" in InfluxDB.
	//
	// Example: {"host": "myhost.mydomain.com", "total_cpus": "2"}
	IndexedDimensions map[string]string

	// UnindexedDimensions capture metadata about the measurement.  They map to
	// "fields" in InfluxDB. Unlikely IndexedDimensions, it's okay to include
	// high cardinality values here, but queries on UnindexedDimensions will be
	// slow.
	//
	// Example: {"requestid": "18af517b-004f-486c-9978-6cf60be7f1e9",
	//           "ipv6": "2001:0db8:0a0b:12f0:0000:0000:0000:0001"}
	UnindexedDimensions map[string]interface{}

	// Gauges represent a snapshot of a constantly shifting measurement at a point
	// in time. During downsampling, we keep the most recent gauge for a given set
	// of dimensions.
	//
	// Example: {"idle": 10.1, "system": 53.3, "user": 36.6}
	Gauges map[string]float64

	// Counts is a map of counts associated with the measurement, keyed to their
	// names. During downsampling, we keep the most recent count for a given
	// set of Dimensions
	//
	// Example: {"swapouts": 512343, "swapins": 64534}
	Counts map[string]int64

	// Increments are like Counts except that they represent deltas during a time
	// period rather than a total count at a point in time. During downsampling,
	// increments will automatically be aggregated into the most recent point,
	// grouped by the Dimensions.
	//
	// Example: {"numerrors": 5}
	Increments map[string]int64
}
