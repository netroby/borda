package borda

import (
	"time"

	"github.com/getlantern/goexpr/isp"
	"github.com/getlantern/goexpr/isp/maxmind"
	"github.com/getlantern/zenodb"
	"gopkg.in/redis.v5"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir string, schemaFile string, aliasesFile string, ispdb string, redisClient *redis.Client, redisCacheSize int, maxWALSize int, walCompressionSize int, numPartitions int, clusterQueryConcurrency int, maxMemory float64) (SaveFunc, *zenodb.DB, error) {
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
		Dir:                     dir,
		SchemaFile:              schemaFile,
		AliasesFile:             aliasesFile,
		EnableGeo:               true,
		ISPProvider:             ispProvider,
		RedisClient:             redisClient,
		RedisCacheSize:          redisCacheSize,
		WALSyncInterval:         5 * time.Second,
		MaxWALSize:              maxWALSize,
		WALCompressionSize:      walCompressionSize,
		MaxMemoryRatio:          maxMemory,
		Passthrough:             true,
		NumPartitions:           numPartitions,
		ClusterQueryConcurrency: clusterQueryConcurrency,
	})
	if err != nil {
		return nil, nil, err
	}
	db.HandleShutdownSignal()

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
