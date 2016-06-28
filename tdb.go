package borda

import (
	"time"

	"github.com/oxtoacart/tdb"
)

// TDBSave creates a SaveFN that saves to an embedded tdb.DB
func TDBSave(dir) (SaveFunc, error) {
	resolution := 5 * time.Minute
	hotPeriod := 10 * time.Minute
	retentionPeriod := 1 * time.Hour

	db := tdb.NewDB(&tdb.DBOpts{
		Dir:       dir,
		BatchSize: 1000,
	})
	err = db.CreateTable("combined", resolution, hotPeriod, retentionPeriod, tdb.DerivedField{
		Name: "error_rate",
		Expr: Avg(Calc("error_count / success_count")),
	})

	return func(m *Measurement) error {
		return db.Insert("combined", &tdb.Point{
			Ts:   m.Ts,
			Dims: m.Dimensions,
			Vals: m.Values,
		})
	}
}
