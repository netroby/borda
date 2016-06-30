package borda

import (
	"time"

	"github.com/dustin/go-humanize"
	"github.com/oxtoacart/tdb"
	. "github.com/oxtoacart/tdb/expr"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir string) (SaveFunc, *tdb.DB, error) {
	resolution := 5 * time.Minute
	hotPeriod := 10 * time.Minute
	retentionPeriod := 1 * time.Hour

	db := tdb.NewDB(&tdb.DBOpts{
		Dir:       dir,
		BatchSize: 1000,
	})
	err := db.CreateTable("combined", resolution, hotPeriod, retentionPeriod, map[string]Expr{
		"success_count": Sum("success_count"),
		"error_count":   Sum("error_count"),
	})
	if err != nil {
		return nil, nil, err
	}
	err = db.CreateTable("proxies", resolution, hotPeriod, retentionPeriod, map[string]Expr{
		"success_count": Sum("success_count"),
		"error_count":   Sum("error_count"),
		"error_rate":    Avg(Div("error_count", Add("success_count", "error_count"))),
	})
	if err != nil {
		return nil, nil, err
	}
	err = db.CreateView("combined", "proxies", "proxy_host")
	if err != nil {
		return nil, nil, err
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			for _, table := range []string{"combined", "proxies"} {
				stats := db.TableStats(table)
				log.Debugf("%v at %v -- Inserted Points: %v   Dropped Points: %v   Hot Keys: %v   Archived Buckets: %v", table, db.Now(table).In(time.UTC), humanize.Comma(stats.InsertedPoints), humanize.Comma(stats.DroppedPoints), humanize.Comma(stats.HotKeys), humanize.Comma(stats.ArchivedBuckets))
			}
		}
	}()

	return func(m *Measurement) error {
		return db.Insert("combined", &tdb.Point{
			Ts:   m.Ts,
			Dims: m.Dimensions,
			Vals: m.Values,
		})
	}, db, nil
}
