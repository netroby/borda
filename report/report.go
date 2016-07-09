package report

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/getlantern/golog"
	"github.com/getlantern/tdb"
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
	sql, err := url.QueryUnescape(req.URL.RawQuery)
	if err != nil {
		badRequest(resp, "Please url encode your sql query")
	}
	if sql == "" {
		badRequest(resp, "Please specify some sql in your query")
		return
	}

	aq, err := h.DB.SQLQuery(sql)
	if err != nil {
		badRequest(resp, err.Error())
		return
	}

	result, err := aq.Run()
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	porcelain := strings.EqualFold("/porcelain", req.URL.Path)

	if !porcelain {
		fmt.Fprintf(resp, "------------- %v ----------------\n", result.Table)
		fmt.Fprintln(resp, sql)
		fmt.Fprintln(resp)
		fmt.Fprintf(resp, "# As Of:      %v\n", result.AsOf)
		fmt.Fprintf(resp, "# Until:      %v\n", result.Until)
		fmt.Fprintf(resp, "# Resolution: %v\n", result.Resolution)
		fmt.Fprintf(resp, "# Group By:   %v\n\n", strings.Join(result.GroupBy, " "))

		fmt.Fprintf(resp, "# Query Runtime:  %v\n\n", result.Stats.Runtime)

		fmt.Fprintln(resp, "# Key Statistics")
		fmt.Fprintf(resp, "#   Scanned:       %v\n", humanize.Comma(result.Stats.Scanned))
		fmt.Fprintf(resp, "#   Filter Pass:   %v\n", humanize.Comma(result.Stats.FilterPass))
		fmt.Fprintf(resp, "#   Read Value:    %v\n", humanize.Comma(result.Stats.ReadValue))
		fmt.Fprintf(resp, "#   Valid:         %v\n", humanize.Comma(result.Stats.DataValid))
		fmt.Fprintf(resp, "#   In Time Range: %v\n\n", humanize.Comma(result.Stats.InTimeRange))

		fmt.Fprintf(resp, "# %-33v", "time")
	}

	// Calculate widths for dimensions
	dimWidths := make(map[string]int, len(result.GroupBy))
	for _, entry := range result.Entries {
		for dim, val := range entry.Dims {
			width := len(fmt.Sprint(val))
			if width > dimWidths[dim] {
				dimWidths[dim] = width
			}
		}
	}

	dimFormats := make([]string, 0, len(dimWidths))
	for _, dim := range result.GroupBy {
		dimFormats = append(dimFormats, "%-"+fmt.Sprint(dimWidths[dim]+4)+"v")
	}

	if !porcelain {
		for i, dim := range result.GroupBy {
			fmt.Fprintf(resp, dimFormats[i], dim)
		}

		for _, field := range result.Fields {
			fmt.Fprintf(resp, "%20v", field.Name)
		}
		fmt.Fprint(resp, "\n")
	}

	numPeriods := int(result.Until.Sub(result.AsOf) / result.Resolution)
	for e, entry := range result.Entries {
		for i := 0; i < numPeriods; i++ {
			fmt.Fprintf(resp, "%-35v", result.Until.Add(-1*time.Duration(i)*result.Resolution).Format(time.RFC1123))
			for i, dim := range result.GroupBy {
				fmt.Fprintf(resp, dimFormats[i], entry.Dims[dim])
			}
			for _, field := range result.Fields {
				fmt.Fprintf(resp, "%20.4f", entry.Fields[field.Name][i].Get())
			}
			if i < numPeriods-1 {
				fmt.Fprint(resp, "\n")
			}
		}
		if e < len(result.Entries)-1 {
			fmt.Fprint(resp, "\n")
		}
	}
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}
