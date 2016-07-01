package report

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/getlantern/golog"
	"github.com/oxtoacart/tdb"
)

var (
	log = golog.LoggerFor("report")
)

type Handler struct {
	DB *tdb.DB
}

// ServeHTTP implements the http.Handler interface and supports querying via
// HTTP.
func (h *Handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(resp, "Method %v not allowed\n", req.Method)
		return
	}

	resp.Header().Set("Content-Type", "text/plain")
	query := req.URL.Query()
	sql := query.Get("sql")
	if sql == "" {
		badRequest(resp, "Please specify some sql")
		return
	}

	fromString := query.Get("from")
	fromOffset, err := time.ParseDuration(fromString)
	if err != nil {
		badRequest(resp, "Error parsing from offset %v: %v", fromString, err)
		return
	}
	now := time.Now()
	from := now.Add(-1 * fromOffset)

	to := now
	toString := query.Get("to")
	toOffset := 0 * time.Second
	if toString != "" {
		toOffset, err = time.ParseDuration(toString)
		if err != nil {
			badRequest(resp, "Error parsing to offset %v: %v", toString, err)
			return
		}
		to = now.Add(-1 * toOffset)
	}
	aq, err := h.DB.SQLQuery(sql)
	if err != nil {
		badRequest(resp, err.Error())
		return
	}
	aq.From(from).To(to)

	result, err := aq.Run()
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	fmt.Fprintf(resp, "------------- %v ----------------\n", result.Table)
	fmt.Fprintln(resp, sql)
	fmt.Fprintln(resp)
	fmt.Fprintf(resp, "# From:        %v\n", result.From)
	fmt.Fprintf(resp, "# To:          %v\n", result.To)
	fmt.Fprintf(resp, "# Resolution:  %v\n\n", result.Resolution)

	fmt.Fprintln(resp, "# Statistics")
	fmt.Fprintf(resp, "# Scanned:     %v\n", humanize.Comma(result.Stats.Scanned))
	fmt.Fprintf(resp, "# Filter Pass: %v\n", humanize.Comma(result.Stats.FilterPass))
	fmt.Fprintf(resp, "# Read Value:  %v\n", humanize.Comma(result.Stats.ReadValue))
	fmt.Fprintf(resp, "# Valid:       %v\n", humanize.Comma(result.Stats.DataValid))
	fmt.Fprintf(resp, "# Included:    %v\n\n", humanize.Comma(result.Stats.Included))

	fmt.Fprintf(resp, "# %-33v", "time")

	// Calculate widths for dimensions
	dimWidths := make(map[string]int, len(result.Dims))
	for _, entry := range result.Entries {
		for dim, val := range entry.Dims {
			width := len(fmt.Sprint(val))
			if width > dimWidths[dim] {
				dimWidths[dim] = width + 4 // padding
			}
		}
	}

	dimFormats := make([]string, 0, len(dimWidths))
	for _, dim := range result.Dims {
		dimFormats = append(dimFormats, "%-"+fmt.Sprint(dimWidths[dim])+"v")
	}

	for i, dim := range result.Dims {
		fmt.Fprintf(resp, dimFormats[i], dim)
	}

	for _, field := range result.FieldOrder {
		fmt.Fprintf(resp, "%20v", field)
	}
	fmt.Fprint(resp, "\n")

	numPeriods := int(result.To.Sub(result.From) / result.Resolution)
	for _, entry := range result.Entries {
		for i := 0; i < numPeriods; i++ {
			fmt.Fprintf(resp, "%-35v", result.To.Add(-1*time.Duration(i)*result.Resolution).Format(time.RFC1123))
			for i, dim := range result.Dims {
				fmt.Fprintf(resp, dimFormats[i], entry.Dims[dim])
			}
			for _, field := range result.FieldOrder {
				fmt.Fprintf(resp, "%20.4f", entry.Fields[field][i].Get())
			}
			fmt.Fprint(resp, "\n")
		}
	}
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}
