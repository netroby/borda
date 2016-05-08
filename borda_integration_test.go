// +build integration_test

package borda

import (
	"os/exec"
	"testing"
	"time"

	"github.com/influxdata/influxdb/client/v2"
	"github.com/stretchr/testify/assert"
)

func TestRealWorldScenario(t *testing.T) {
	// Drop the database
	out, err := exec.Command("./drop_database.sh").Output()
	assert.NoError(t, err, "Unable to drop database: %v", string(out))

	c, err := NewCollector(&Options{
		Dimensions:     []string{"client_error", "proxy_error", "client", "proxy"},
		InfluxURL:      "http://localhost:8086",
		DBName:         "lantern",
		User:           "test",
		Pass:           "test",
		BatchSize:      1000,
		MaxBatchWindow: 25 * time.Millisecond,
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

	// Scenario 1 - Client unable to dial proxy because proxy is overloaded
	c.Submit(&Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"client_error":       "ChainedProxyDialError",
			"client_error_count": 1,
			"client":             "a",
			"proxy":              "127.0.0.1",
			"request_id":         "r1",
		},
	})
	c.Submit(&Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"proxy":    "127.0.0.1",
			"load_avg": 0.99,
		},
	})

	// Scenario 2 - Request rejected because of missing auth token
	c.Submit(&Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"client_error":       "ChainedProxyBadGateway",
			"client_error_count": 1,
			"client":             "b",
			"proxy":              "127.0.0.1",
			"request_id":         "r2",
		},
	})
	c.Submit(&Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"proxy_error":       "MissingAuthToken",
			"proxy_error_count": 1,
			"client":            "b",
			"proxy":             "127.0.0.1",
			"request_id":        "r2",
		},
	})

	time.Sleep(2 * time.Second)

	query := client.NewQuery(`SELECT * FROM lantern."default".health_250ms group by client`, c.(*collector).DBName, "ns")
	response, err := c.(*collector).influx.Query(query)
	if assert.NoError(t, err, "Unable to execute query") && assert.NoError(t, response.Error(), "Error on response") {
		if assert.Len(t, response.Results, 1, "Wrong number of results") {
			assert.Len(t, response.Results[0].Series, 3, "Wrong number of series")
		}
	}

	out, err = exec.Command("./query.sh").Output()
	if assert.NoError(t, err, "Unable to run queries: %v", string(out)) {
		t.Log(string(out))
	}
}
