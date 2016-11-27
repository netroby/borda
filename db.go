package borda

import (
	"time"

	"github.com/getlantern/goexpr/isp"
	"github.com/getlantern/goexpr/isp/maxmind"
	"github.com/getlantern/zenodb"
	"github.com/getlantern/zenodb/sql"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir string, schemaFile string, ispdb string, maxWALAge time.Duration, walCompressionAge time.Duration, numPartitions int) (SaveFunc, *zenodb.DB, error) {
	sql.RegisterUnaryDIMFunction("HOSTNAME", BuildHostname)
	var ispProvider isp.Provider
	var ispErr error
	if ispdb != "" {
		log.Debugf("Using maxmind db at %v", ispdb)
		ispProvider, ispErr = maxmind.NewProvider(ispdb)
		if ispErr != nil {
			return nil, nil, ispErr
		}
	}

	db, err := zenodb.NewDB(&zenodb.DBOpts{
		Dir:               dir,
		SchemaFile:        schemaFile,
		ISPProvider:       ispProvider,
		WALSyncInterval:   5 * time.Second,
		MaxWALAge:         maxWALAge,
		WALCompressionAge: walCompressionAge,
		Passthrough:       true,
		PartitionBy:       []string{"client_ip", "proxy_host", "app_version", "error"},
		NumPartitions:     numPartitions,
	})
	if err != nil {
		return nil, nil, err
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			for name := range db.AllTableStats() {
				db.PrintTableStats(name)
			}
		}
	}()

	return func(m *Measurement) error {
		return db.Insert("inbound",
			time.Now(), // use now so that clients with bad clocks don't throw things off m.Ts,
			m.Dimensions,
			m.Values)
	}, db, nil
}
