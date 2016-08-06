package report

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/getlantern/golog"
	"github.com/getlantern/tibsdb"
)

var (
	log = golog.LoggerFor("report")
)

type Handler struct {
	DB *tibsdb.DB
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
	}

	// Calculate widths for dimensions and fields
	dimWidths := make([]int, len(result.GroupBy))
	fieldWidths := make([]int, len(result.FieldNames))

	for i, dim := range result.GroupBy {
		width := len(dim)
		if width > dimWidths[i] {
			dimWidths[i] = width
		}
	}

	for i, field := range result.FieldNames {
		width := len(field)
		if width > fieldWidths[i] {
			fieldWidths[i] = width
		}
	}

	for _, row := range result.Rows {
		for i, val := range row.Dims {
			width := len(fmt.Sprint(val))
			if width > dimWidths[i] {
				dimWidths[i] = width
			}
		}

		for i, val := range row.Values {
			width := len(fmt.Sprint(val))
			if width > fieldWidths[i] {
				fieldWidths[i] = width
			}
		}
	}

	// Create formats for dims and fields
	dimFormats := make([]string, 0, len(dimWidths))
	fieldLabelFormats := make([]string, 0, len(fieldWidths))
	fieldFormats := make([]string, 0, len(fieldWidths))
	for _, width := range dimWidths {
		dimFormats = append(dimFormats, "%-"+fmt.Sprint(width+4)+"v")
	}
	for _, width := range fieldWidths {
		fieldLabelFormats = append(fieldLabelFormats, "%"+fmt.Sprint(width+4)+"v")
		fieldFormats = append(fieldFormats, "%"+fmt.Sprint(width+4)+".4f")
	}

	if !porcelain {
		// Print header row
		fmt.Fprintf(resp, "# %-33v", "time")
		for i, dim := range result.GroupBy {
			fmt.Fprintf(resp, dimFormats[i], dim)
		}
		for i, field := range result.FieldNames {
			fmt.Fprintf(resp, fieldLabelFormats[i], field)
		}
		fmt.Fprint(resp, "\n")
	}

	for _, row := range result.Rows {
		fmt.Fprintf(resp, "%-35v", result.Until.Add(-1*time.Duration(row.Period)*result.Resolution).Format(time.RFC1123))
		for i, dim := range row.Dims {
			fmt.Fprintf(resp, dimFormats[i], dim)
		}
		for i, val := range row.Values {
			fmt.Fprintf(resp, fieldFormats[i], val)
		}
		fmt.Fprint(resp, "\n")
	}
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}
