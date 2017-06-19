package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/getlantern/errors"
	"github.com/getlantern/golog"
	"github.com/getlantern/ops"
	"github.com/getlantern/zenodb/rpc"
	"github.com/oxtoacart/bpool"
)

var (
	log = golog.LoggerFor("borda.client")

	bordaURL = "https://borda.getlantern.org/measurements"

	bufferPool = bpool.NewBufferPool(100)
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
	Values map[string]Val `json:"values,omitempty"`

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
	Dimensions json.RawMessage `json:"dimensions,omitempty"`

	dimensions map[string]interface{}
}

// Options provides configuration options for borda clients
type Options struct {
	// BatchInterval specifies how frequent to report to borda
	BatchInterval time.Duration

	// HTTP Client used to report to Borda
	HTTPClient *http.Client

	// RPC Client used to report to Borda
	RPCClient rpc.Client

	// BeforeSubmit is an optional callback that gets called before submitting a
	// batch to borda. The callback should not modify the values and dimensions.
	BeforeSubmit func(name string, key string, ts time.Time, values map[string]Val, dimensions map[string]interface{})
}

// Submitter is a functon that submits measurements to borda. If the measurement
// was successfully queued for submission, this returns nil.
type Submitter func(values map[string]Val, dimensions map[string]interface{}) error

type submitter func(key string, ts time.Time, values map[string]Val, dimensions map[string]interface{}, jsonDimensions []byte) error

// Client is a client that submits measurements to the borda server.
type Client struct {
	hc           *http.Client
	rc           rpc.Client
	options      *Options
	buffers      map[int]map[string]*Measurement
	submitters   map[int]submitter
	nextBufferID int
	mx           sync.Mutex
}

// NewClient creates a new borda client.
func NewClient(opts *Options) *Client {
	if opts == nil {
		opts = &Options{}
	}
	if opts.BatchInterval <= 0 {
		log.Debugf("BatchInterval has to be greater than zero, defaulting to 5 minutes")
		opts.BatchInterval = 5 * time.Minute
	}
	if opts.HTTPClient == nil && opts.RPCClient == nil {
		// Default to HTTPClient
		opts.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ClientSessionCache: tls.NewLRUClientSessionCache(100),
				},
			},
		}
	}
	if opts.BeforeSubmit == nil {
		opts.BeforeSubmit = func(name string, key string, ts time.Time, values map[string]Val, dimensions map[string]interface{}) {
		}
	}

	b := &Client{
		hc:         opts.HTTPClient,
		rc:         opts.RPCClient,
		options:    opts,
		buffers:    make(map[int]map[string]*Measurement),
		submitters: make(map[int]submitter),
	}

	go b.sendPeriodically()
	return b
}

// DefaultClient creates a new Client that connects to borda.getlantern.org
// using gRPC if possible, or falling back to HTTPS if it can't dial out with
// gRPC.
func DefaultClient(batchInterval time.Duration, maxBufferSize int) *Client {
	log.Debugf("Creating borda client that submits every %v", batchInterval)

	opts := &Options{
		BatchInterval: batchInterval,
	}

	clientSessionCache := tls.NewLRUClientSessionCache(10000)
	clientTLSConfig := &tls.Config{
		ServerName:         "borda.getlantern.org",
		ClientSessionCache: clientSessionCache,
	}

	rc, err := rpc.Dial("borda.getlantern.org:17712", &rpc.ClientOpts{
		Dialer: func(addr string, timeout time.Duration) (net.Conn, error) {
			log.Debug("Dialing borda with gRPC")
			conn, dialErr := net.DialTimeout("tcp", addr, timeout)
			if dialErr != nil {
				return nil, dialErr
			}
			tlsConn := tls.Client(conn, clientTLSConfig)
			handshakeErr := tlsConn.Handshake()
			if handshakeErr != nil {
				log.Errorf("Error TLS handshaking with borda: %v", handshakeErr)
				conn.Close()
			}
			return tlsConn, handshakeErr
		},
	})
	if err != nil {
		log.Errorf("Unable to dial borda, will not use gRPC: %v", err)
	} else {
		log.Debug("Using gRPC to communicate with borda")
		opts.RPCClient = rc
	}

	return NewClient(opts)
}

// EnableOpsReporting registers a reporter with the ops package that reports op
// successes and failures to borda under the given measurement name.
func (c *Client) EnableOpsReporting(name string, maxBufferSize int) {
	reportToBorda := c.ReducingSubmitter(name, maxBufferSize)

	ops.RegisterReporter(func(failure error, ctx map[string]interface{}) {
		values := map[string]Val{}
		if failure != nil {
			values["error_count"] = Float(1)
		} else {
			values["success_count"] = Float(1)
		}

		reportErr := reportToBorda(values, ctx)
		if reportErr != nil {
			log.Errorf("Error reporting error to borda: %v", reportErr)
		}
	})
}

// ReducingSubmitter returns a Submitter whose measurements are reduced based on
// their types. name specifies the name of the measurements and
// maxBufferSize specifies the maximum number of distinct measurements to buffer
// within the BatchInterval. Anything past this is discarded.
func (c *Client) ReducingSubmitter(name string, maxBufferSize int) Submitter {
	if maxBufferSize <= 0 {
		log.Debugf("maxBufferSize has to be greater than zero, defaulting to 1000")
		maxBufferSize = 1000
	}
	c.mx.Lock()
	defer c.mx.Unlock()
	bufferID := c.nextBufferID
	c.nextBufferID++
	submitter := func(key string, ts time.Time, values map[string]Val, dimensions map[string]interface{}, jsonDimensions []byte) error {
		buffer := c.buffers[bufferID]
		if buffer == nil {
			// Lazily initialize buffer
			buffer = make(map[string]*Measurement)
			c.buffers[bufferID] = buffer
		}
		existing, found := buffer[key]
		if found {
			for key, value := range values {
				existing.Values[key] = value.Merge(existing.Values[key])
			}
			if ts.After(existing.Ts) {
				existing.Ts = ts
			}
		} else if len(buffer) == maxBufferSize {
			return errors.New("Exceeded max buffer size, discarding measurement")
		} else {
			buffer[key] = &Measurement{
				Name:       name,
				Ts:         ts,
				Values:     values,
				Dimensions: jsonDimensions,
				dimensions: dimensions,
			}
		}
		return nil
	}
	c.submitters[bufferID] = submitter

	return func(values map[string]Val, dimensions map[string]interface{}) error {
		// Convert metrics to values
		for dim, val := range dimensions {
			metric, ok := val.(Val)
			if ok {
				delete(dimensions, dim)
				values[dim] = metric
			}
		}

		jsonDimensions, encodeErr := json.Marshal(dimensions)
		if encodeErr != nil {
			return errors.New("Unable to marshal dimensions: %v", encodeErr)
		}
		key := string(jsonDimensions)
		ts := time.Now()
		c.options.BeforeSubmit(name, key, ts, values, dimensions)
		c.mx.Lock()
		err := submitter(key, ts, values, dimensions, jsonDimensions)
		c.mx.Unlock()
		return err
	}
}

func (c *Client) sendPeriodically() {
	log.Debugf("Reporting to Borda every %v", c.options.BatchInterval)
	for range time.NewTicker(c.options.BatchInterval).C {
		c.Flush()
	}
}

// Flush flushes any currently buffered data.
func (c *Client) Flush() {
	c.mx.Lock()
	currentBuffers := c.buffers
	// Clear out buffers
	c.buffers = make(map[int]map[string]*Measurement, len(c.buffers))
	c.mx.Unlock()

	// Count measurements
	numMeasurements := 0
	for _, buffer := range currentBuffers {
		numMeasurements += len(buffer)
	}
	if numMeasurements == 0 {
		log.Debug("Nothing to report")
		return
	}

	// Make batch
	batch := make(map[string][]*Measurement)
	for _, buffer := range currentBuffers {
		for _, m := range buffer {
			name := m.Name
			batch[name] = append(batch[name], m)
		}
	}

	log.Debugf("Attempting to report %d measurements to Borda", numMeasurements)
	numInserted, err := c.doSendBatch(batch)
	log.Debugf("Sent %d measurements", numInserted)
	if err != nil {
		log.Errorf("Error sending batch: %v", err)
	}
}

func (c *Client) doSendBatch(batch map[string][]*Measurement) (int, error) {
	if c.rc != nil {
		log.Debug("Sending batch with RPC")
		return c.doSendBatchRPC(batch)
	}
	log.Debug("Sending batch with HTTP")
	return c.doSendBatchHTTP(batch)
}

func (c *Client) doSendBatchHTTP(batchByName map[string][]*Measurement) (int, error) {
	numInserted := 0
	var batch []*Measurement
	for _, measurements := range batchByName {
		numInserted += len(measurements)
		batch = append(batch, measurements...)
	}
	buf := bufferPool.Get()
	defer bufferPool.Put(buf)
	err := json.NewEncoder(buf).Encode(batch)
	if err != nil {
		return 0, log.Errorf("Unable to report measurements: %v", err)
	}

	req, decErr := http.NewRequest(http.MethodPost, bordaURL, buf)
	if decErr != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 201:
		return numInserted, nil
	case 400:
		errorMsg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, fmt.Errorf("Borda replied with 400, but error message couldn't be read: %v", err)
		}
		return 0, fmt.Errorf("Borda replied with the error: %v", string(errorMsg))
	default:
		return 0, fmt.Errorf("Borda replied with error %d", resp.StatusCode)
	}
}

func (c *Client) doSendBatchRPC(batch map[string][]*Measurement) (int, error) {
	numInserted := 0
	for _, measurements := range batch {
		// TODO: right now we send everything to "inbound", might be nice to
		// separate streams where we can.
		inserter, err := c.rc.NewInserter(context.Background(), "inbound")
		if err != nil {
			return numInserted, fmt.Errorf("Unable to get inserter: %v", err)
		}
		for _, m := range measurements {
			err = inserter.Insert(m.Ts, m.dimensions, func(cb func(string, interface{})) {
				for key, val := range m.Values {
					cb(key, val.Get())
				}
			})
			if err != nil {
				inserter.Close()
				return numInserted, fmt.Errorf("Error inserting: %v", err)
			}
		}
		report, err := inserter.Close()
		if err != nil {
			return numInserted, fmt.Errorf("Error closing inserter: %v", err)
		}
		numInserted += report.Succeeded
	}
	return numInserted, nil
}
