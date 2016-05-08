// +build omit

package borda

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRealWorldScenario(t *testing.T) {
	// Drop the database
	out, err := exec.Command("./drop_database.sh").Output()
	if !assert.NoError(t, err, "Unable to drop database: %v", out) {
		return
	}

	c, err := NewCollector(&Options{
		Dimensions:    []string{"dim_string", "dim_int"},
		InfluxURL:     "http://localhost:8086",
		DBName:        "lantern",
		User:          "test",
		Pass:          "test",
		BatchSize:     1,
		MaxRetries:    100,
		RetryInterval: 250 * time.Millisecond,
	})
	if !assert.NoError(t, err, "Unable to create Collector") {
		return
	}

	// Simulate measurements from clients and servers
	// All measurements go to the same key "health" so that they can be correlated
	m := &Measurement{
		Name: "health",
		Ts:   time.Now(),
		Fields: map[string]interface{}{
			"dim_string":   "a",
			"dim_int":      1,
			"field_int":    1,
			"field_float":  1.1,
			"field_bool":   true,
			"field_string": "stringy",
		},
	}

	// Try to start using database before it even exists (should yield some errors
	// in log)
	c.Submit(m)
	time.Sleep(1 * time.Second)

	// Create the database
	out, err = exec.Command("./create_database.sh").Output()
	if !assert.NoError(t, err, "Unable to create database: %v", out) {
		return
	}

	time.Sleep(1 * time.Second)
}
