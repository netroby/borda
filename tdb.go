package borda

import (
	"time"

	"github.com/dustin/go-humanize"
	"github.com/oxtoacart/tdb"
	. "github.com/oxtoacart/tdb/expr"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir string) (SaveFunc, error) {
	resolution := 1 * time.Minute
	hotPeriod := 2 * time.Minute
	retentionPeriod := 1 * time.Hour

	db := tdb.NewDB(&tdb.DBOpts{
		Dir:       dir,
		BatchSize: 1000,
	})
	err := db.CreateTable("combined", resolution, hotPeriod, retentionPeriod, tdb.DerivedField{
		Name: "error_rate",
		Expr: Avg(Calc("error_count / success_count")),
	})
	if err != nil {
		return nil, err
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			stats := db.TableStats("combined")
			log.Debugf("Hot Keys: %v     Archived Buckets: %v", humanize.Comma(stats.HotKeys), humanize.Comma(stats.ArchivedBuckets))
		}
	}()

	return func(m *Measurement) error {
		return db.Insert("combined", &tdb.Point{
			Ts:   m.Ts,
			Dims: m.Dimensions,
			Vals: m.Values,
		})
	}, nil
}
