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
		Fields: map[string]Expr{
			"success_count": Sum("success_count"),
			"error_count":   Sum("error_count"),
			"error_rate":    Div(Sum("error_count"), Add(Sum("error_count"), Sum("success_count"))),
		},
		OrderBy: map[string]Order{
			"error_rate": ORDER_DESC,
		},
	}

	q := &Query{
		Table: "proxies",
		From:  time.Now().Add(-1 * time.Hour),
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
		totalErrorRate += row.Totals["error_rate"].Get()
	}
	avgErrorRate := totalErrorRate / float64(len(result))

	fmt.Fprintln(resp, "# ---- Proxies by Error Rate ----")
	fmt.Fprintf(resp, "# Average error rate: %f\n\n", avgErrorRate)
	fmt.Fprintln(resp, "#    proxy_host      errors    requests  error_rate")
	for _, row := range result {
		fmt.Fprintf(resp, "%15v%12.4f%12.4f%12.4f\n", row.Dims["proxy_host"], row.Totals["error_count"].Get(), row.Totals["success_count"].Get()+row.Totals["error_count"].Get(), row.Totals["error_rate"].Get())
	}
}
