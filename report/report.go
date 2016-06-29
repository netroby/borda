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

	resp.Header().Set("Content-Type", "text/plain")
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
		Dims:       []string{"proxy_host"},
		Resolution: 15 * time.Minute,
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
				Name: "total_success_count",
				Expr: Calc("success_count"),
			},
			DerivedField{
				Name: "total_error_count",
				Expr: Calc("error_count"),
			},
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
		From:   time.Now().Add(-1 * time.Hour),
	}

	result, err := aq.Run(h.DB, q)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	// Calculate the overall average error rate
	totalErrorRate := float64(0)
	for _, row := range result {
		totalErrorRate += row.Summaries["avg_error_rate"]
	}
	avgErrorRate := totalErrorRate / float64(len(result))

	fmt.Fprintln(resp, "---- Proxies by Error Rate ----")
	fmt.Fprintf(resp, "Average error rate: %f\n\n", avgErrorRate)
	for _, row := range result {
		fmt.Fprintf(resp, "%v : %f / %f -> %f\n", row.Dims["proxy_host"], row.Summaries["total_error_count"], row.Summaries["total_success_count"], row.Summaries["avg_error_rate"])
	}
}
