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

	resp.Header().Set("Content-Type", "text/tsv")
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
		Dims: []string{"proxy_host"},
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
		Summaries: []DerivedField{
			DerivedField{
				Name: "avg_error_rate",
				Expr: Avg(Calc("error_rate")),
			},
		},
		OrderBy: map[string]Order{
			"avg_error_rate": ORDER_DESC,
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

	// Take only the top 20
	if len(result) > 20 {
		result = result[:20]
	}

	numPeriods := 0
	// Print headers
	fmt.Fprint(resp, "# ")
	for _, row := range result {
		numPeriods = row.NumPeriods
		fmt.Fprint(resp, row.Dims["proxy_host"])
		fmt.Fprint(resp, "\t")
	}
	fmt.Fprint(resp, "\n")

	for i := 0; i < numPeriods; i++ {
		fmt.Fprint(resp, i)
		for _, row := range result {
			fmt.Fprint(resp, row.Fields["error_rate"][i])
			fmt.Fprint(resp, "\t")
		}
		fmt.Fprint(resp, "\n")
	}
}
