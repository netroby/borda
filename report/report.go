package report

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getlantern/golog"
	. "github.com/oxtoacart/tdb"
	. "github.com/oxtoacart/tdb/expr"
)

var (
	log = golog.LoggerFor("report")
)

type Handler struct {
	DB *DB
}

// ServeHTTP implements the http.Handler interface and supports getting reports
// via HTTP.
func (h *Handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(resp, "Method %v not allowed\n", req.Method)
		return
	}

	resp.Header().Set("Content-Type", "text/csv")
	switch strings.ToLower(req.URL.Path)[1:] {
	case "byerrorrate":
		h.byErrorRate(resp)
	default:
		log.Errorf("No report for path %v", req.URL.Path)
		http.NotFound(resp, req)
		return
	}

}

func (h *Handler) byErrorRate(resp http.ResponseWriter) {
	aq := &AggregateQuery{
		Resolution: 2 * time.Minute,
		Dims:       []string{"proxy_host"},
		Fields: []DerivedField{
			DerivedField{
				Name: "success_count",
				Expr: Calc("success_count"),
			},
			DerivedField{
				Name: "error_count",
				Expr: Calc("error_count"),
			},
			DerivedField{
				Name: "error_rate",
				Expr: Avg(Calc("success_count > 0 ? error_count / success_count")),
			},
		},
		OrderBy: map[string]Order{
			"error_rate": ORDER_DESC,
		},
	}

	q := &Query{
		Table:  "combined",
		Fields: []string{"success_count", "error_count"},
		From:   time.Now().Add(-10 * time.Minute),
	}

	result, err := aq.Run(h.DB, q)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	fmt.Fprintln(resp, "proxy_host,period,success_count,error_count,error_rate,")
	for _, row := range result {
		for i := 0; i < row.NumPeriods; i++ {
			fmt.Fprint(resp, row.Dims[0])
			fmt.Fprint(resp, ",?,")
			for _, vals := range row.Fields {
				fmt.Fprint(resp, vals[i])
				fmt.Fprint(resp, ",")
			}
			fmt.Fprint(resp, "\n")
		}
	}
}
