// +build integration_test

package borda

import (
	"fmt"
	"math/rand"
	"os/exec"
	"testing"
	"time"

	"code.google.com/p/go-uuid/uuid"

	"github.com/stretchr/testify/assert"
)

const (
	numProxies       = 20
	numClients       = 200
	proxiesPerClient = 2
)

func init() {
	// Always use the same random seed
	rand.Seed(1)
}

// TestRealWorldScenario simulates real-world scenarios of clients and servers.
func TestRealWorldScenario(t *testing.T) {
	// Drop the database
	out, err := exec.Command("./drop_database.sh").Output()
	if err != nil {
		log.Errorf("Unable to drop database: %v", string(out))
	}

	c, err := NewCollector(&Options{
		Dimensions:     []string{"request_id", "client_error", "proxy_error", "client", "proxy"},
		InfluxURL:      "http://localhost:8086",
		DBName:         "lantern",
		User:           "test",
		Pass:           "test",
		BatchSize:      1000,
		MaxBatchWindow: 250 * time.Millisecond,
		MaxRetries:     100,
		RetryInterval:  250 * time.Millisecond,
	})
	if !assert.NoError(t, err, "Unable to create Collector") {
		return
	}

	// Create the database
	out, err = exec.Command("./create_database.sh").Output()
	if !assert.NoError(t, err, "Unable to create database: %v", string(out)) {
		return
	}

	// Wait for continuous queries to initialize
	time.Sleep(5 * time.Second)

	// Simulate some clients and proxies
	proxies := make([]*proxy, 0, numProxies)
	for i := 0; i < numProxies; i++ {
		proxy := &proxy{
			c:  c,
			ip: fmt.Sprintf("43.25.23.%d", i),
		}
		go proxy.run()
		proxies = append(proxies, proxy)
	}

	for i := 0; i < numClients; i++ {
		clientProxies := make([]*proxy, 0, proxiesPerClient)
		for j := 0; j < proxiesPerClient; j++ {
			clientProxies = append(clientProxies, proxies[(i+j)%numProxies])
		}
		go runClient(fmt.Sprintf("client_%d", i), c, clientProxies)
	}

	// Wait for some data to get generated
	time.Sleep(5 * time.Minute)

	out, err = exec.Command("./drop_cqs.sh").Output()
	assert.NoError(t, err, "Unable to drop continuous queries: %v", string(out))

	out, err = exec.Command("./query.sh").Output()
	if assert.NoError(t, err, "Unable to run queries: %v", string(out)) {
		t.Log(string(out))
	}
}

const (
	result_none  = 0
	result_ok    = 200
	result_error = 503
)

type request struct {
	id     string
	client string
	result chan int
}

type proxy struct {
	c        Collector
	ip       string
	requests chan *request
	loadAvg  float64
}

func (p *proxy) run() {
	p.requests = make(chan *request)
	p.updateLoadAvg()
	timer := time.NewTimer(25 * time.Millisecond)
	for {
		select {
		case req := <-p.requests:
			if p.loadAvg > 0.9 && rand.Float64() > 0.5 {
				// Simulate no response
				req.result <- result_none
			} else if rand.Float64() > 0.9 {
				// Simulate error connecting downstream
				req.result <- result_error
				p.c.Submit(&Measurement{
					Name: "health",
					Ts:   time.Now(),
					Fields: map[string]interface{}{
						"proxy_error":       1000002,
						"proxy_error_count": 1,
						"client":            req.client,
						"proxy":             p.ip,
						"request_id":        req.id,
					},
				})
			} else if rand.Float64() > 0.9 {
				// Simulate timeout by doing nothing
			} else {
				req.result <- result_ok
				p.c.Submit(&Measurement{
					Name: "health",
					Ts:   time.Now(),
					Fields: map[string]interface{}{
						"proxy_success_count": 1,
						"client":              req.client,
						"proxy":               p.ip,
						"request_id":          req.id,
					},
				})
			}
		case <-timer.C:
			p.updateLoadAvg()
			timer.Reset(25 * time.Millisecond)
		}
	}
}

func (p *proxy) updateLoadAvg() {
	p.loadAvg = rand.Float64()
	p.c.Submit(&Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"proxy":    p.ip,
			"load_avg": p.loadAvg,
		},
	})
}

func runClient(id string, c Collector, proxies []*proxy) {
	for {
		time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
		proxy := proxies[rand.Intn(len(proxies))]
		req := &request{
			id:     uuid.NewRandom().String(),
			client: id,
			result: make(chan int),
		}
		proxy.requests <- req
		select {
		case result := <-req.result:
			switch result {
			case result_ok:
				c.Submit(&Measurement{
					Name: "health",
					Ts:   time.Now(),
					Fields: map[string]interface{}{
						"client_success_count": 1,
						"client":               id,
						"proxy":                proxy.ip,
						"request_id":           req.id,
					},
				})
			case result_none:
				c.Submit(&Measurement{
					Name: "health",
					Ts:   time.Now(),
					Fields: map[string]interface{}{
						"client_error":       2,
						"client_error_count": 1,
						"client":             id,
						"proxy":              proxy.ip,
						"request_id":         req.id,
					},
				})
			case result_error:
				c.Submit(&Measurement{
					Name: "health",
					Ts:   time.Now(),
					Fields: map[string]interface{}{
						"client_error":       3,
						"client_error_count": 1,
						"client":             id,
						"proxy":              proxy.ip,
						"request_id":         req.id,
					},
				})
			}
		case <-time.After(25 * time.Millisecond):
			c.Submit(&Measurement{
				Name: "health",
				Ts:   time.Now(),
				Fields: map[string]interface{}{
					"client_error":       1,
					"client_error_count": 1,
					"client":             id,
					"proxy":              proxy.ip,
					"request_id":         req.id,
				},
			})
		}
	}
}
