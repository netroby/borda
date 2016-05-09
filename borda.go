package borda

import (
	"fmt"
	"math"
	"time"

	"github.com/getlantern/eventual"
	"github.com/getlantern/golog"
	"github.com/influxdata/influxdb/client/v2"
)

var (
	log = golog.LoggerFor("borda")
)

const (
	FIELD_TYPE_INT    = 0
	FIELD_TYPE_FLOAT  = 1
	FIELD_TYPE_BOOL   = 2
	FIELD_TYPE_STRING = 3
)

// Measurement represents a measurement at a point in time. It maps to a "point"
// in InfluxDB.
type Measurement struct {
	// Name is the name of the measurement (e.g. cpu_usage). It maps to the "key"
	// in "InfluxDB".
	Name string `json:"name"`

	// Ts records the time of the measurement.
	Ts time.Time `json:"ts,omitempty"`

	// Fields captures key/value pairs with details of the measurement. It maps to
	// "tags" and "fields" in InfluxDB depending on which fields have been
	// configured as Dimensions on the Collector.
	//
	// Example: { "requestid": "18af517b-004f-486c-9978-6cf60be7f1e9",
	//            "ipv6": "2001:0db8:0a0b:12f0:0000:0000:0000:0001",
	//            "host": "myhost.mydomain.com",
	//            "total_cpus": "2",
	//            "cpu_idle": 10.1,
	//            "cpu_system": 53.3,
	//            "cpu_user": 36.6,
	//            "num_errors": 67,
	//            "connected_to_internet": true }
	Fields map[string]interface{} `json:"fields,omitempty"`
}

type Collector interface {
	// Submit submits a measurement to the Collector
	Submit(*Measurement)

	// Wait waits for the Collector to finish running and returns the error that
	// caused the Collector to terminate.
	Wait() error
}

type Options struct {
	// Dimensions identifies which fields should be treated as dimensions (tags
	// in InfluxDB).
	Dimensions []string

	// InfluxURL identifies the url to the InfluxDB server
	InfluxURL string

	// DBName identifies the name of the InfluxDB database
	DBName string

	// User is the InfluxDB username
	User string

	// Pass is the InfluxDB password
	Pass string

	// BatchSize is the number of measurements to include in a batch before
	// writing it. If BatchSize is not specified, it defaults to 1000.
	BatchSize int

	// MaxBatchWindow is the maximum amount of time to wait before writing a
	// batch of measurements. If MaxBatchWindow is reached before BatchSize, the
	// Collector will write the batch of Measurements anyway. If MaxBatchWindow is
	// unspecified, this feature is not used.
	MaxBatchWindow time.Duration

	// MaxRetries caps the number of times that we retry a batch. Defaults to 10.
	MaxRetries int

	// RetryInterval specifies the amount of time to wait before retrying a batch.
	// Defaults to 5 seconds.
	RetryInterval time.Duration
}

type collector struct {
	*Options
	dimensionNames map[string]bool
	in             chan *Measurement
	finalError     eventual.Value
	influx         client.Client
}

// NewCollector creates and starts a new Collector
func NewCollector(opts *Options) (Collector, error) {
	if opts.BatchSize == 0 {
		opts.BatchSize = 1000
	}
	if opts.MaxBatchWindow == 0 {
		opts.MaxBatchWindow = time.Duration(math.MaxInt64)
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 10
	}
	if opts.RetryInterval == 0 {
		opts.RetryInterval = 5 * time.Second
	}

	c := &collector{
		Options:        opts,
		dimensionNames: make(map[string]bool, len(opts.Dimensions)),
		in:             make(chan *Measurement, opts.BatchSize*2),
		finalError:     eventual.NewValue(),
	}
	for _, dim := range opts.Dimensions {
		c.dimensionNames[dim] = true
	}

	var err error
	c.influx, err = client.NewHTTPClient(client.HTTPConfig{
		Addr:     c.InfluxURL,
		Username: c.User,
		Password: c.Pass,
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to create InfluxDB client: %v", err)
	}

	go c.run()

	return c, nil
}

func (c *collector) Submit(m *Measurement) {
	c.in <- m
}

func (c *collector) Wait() error {
	err, _ := c.finalError.Get(-1)
	if err != nil {
		return err.(error)
	}
	return nil
}

// Run runs the Collector. This method returns an error if something goes wrong
// wrong while starting the Collector.   and an eventual runError on which the caller can block to
// find out if anything goes wrong while running.
func (c *collector) run() {
	timer := time.NewTimer(c.MaxBatchWindow)
	var batch client.BatchPoints
	batchSize := 0

	newBatch := func() error {
		var err error
		batch, err = client.NewBatchPoints(client.BatchPointsConfig{
			Database: c.DBName,
		})
		if err != nil {
			return fmt.Errorf("Unable to create batch: %v", err)
		}
		batchSize = 0
		return nil
	}

	commitBatch := func() error {
		defer timer.Reset(c.MaxBatchWindow)
		if batchSize == 0 {
			return nil
		}
		retries := 0
		for {
			err := c.influx.Write(batch)
			if err == nil {
				return newBatch()
			}
			if retries >= c.MaxRetries {
				return fmt.Errorf("Unable to commit batch, not retrying: %v", err)
			}
			log.Errorf("Unable to commit batch, retrying: %v", err)
			retries++
			time.Sleep(c.RetryInterval)
		}
	}

	err := newBatch()
	if err != nil {
		c.terminate(fmt.Errorf("Unable to create batch: %v", err))
		return
	}

	for {
		select {
		case m := <-c.in:
			// Create a point and add to batch
			tags := make(map[string]string, len(c.Dimensions))
			fields := make(map[string]interface{}, len(m.Fields))
			for key, value := range m.Fields {
				if value != nil && value != "" {
					if c.dimensionNames[key] {
						var stringValue string
						switch v := value.(type) {
						case string:
							stringValue = v
						default:
							stringValue = fmt.Sprint(v)
						}
						tags[key] = stringValue
						fields["_"+key] = value
					} else {
						fields[key] = value
					}
				}
			}
			point, err := client.NewPoint(m.Name, tags, fields, m.Ts)
			if err != nil {
				log.Errorf("Unable to add point to batch: %v", err)
				continue
			}
			batch.AddPoint(point)
			batchSize++
			if batchSize == c.BatchSize {
				err := commitBatch()
				if err != nil {
					c.terminate(err)
					return
				}
			}
		case <-timer.C:
			err := commitBatch()
			if err != nil {
				c.terminate(err)
				return
			}
		}
	}
}

func (c *collector) terminate(err error) {
	c.finalError.Set(err)
	return
}
