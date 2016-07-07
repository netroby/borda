package borda

import (
	"time"

	"github.com/dustin/go-humanize"
	"github.com/getlantern/tdb"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir string, schemaFile string) (SaveFunc, *tdb.DB, error) {
	db, err := tdb.NewDB(&tdb.DBOpts{
		Dir:        dir,
		SchemaFile: schemaFile,
		BatchSize:  1000,
	})
	if err != nil {
		return nil, nil, err
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			for name, stats := range db.AllTableStats() {
				log.Debugf("%v at %v -- Inserted Points: %v   Dropped Points: %v   Hot Keys: %v   Archived Buckets: %v   Expired Keys: %v", name, db.Now(name).In(time.UTC), humanize.Comma(stats.InsertedPoints), humanize.Comma(stats.DroppedPoints), humanize.Comma(stats.HotKeys), humanize.Comma(stats.ArchivedBuckets), humanize.Comma(stats.ExpiredKeys))
			}
		}
	}()

	return func(m *Measurement) error {
		return db.Insert("inbound", &tdb.Point{
			Ts:   m.Ts,
			Dims: m.Dimensions,
			Vals: m.Values,
		})
	}, db, nil
}
