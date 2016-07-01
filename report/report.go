package report

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getlantern/golog"
	. "github.com/oxtoacart/tdb"
	"github.com/oxtoacart/tdb/expr"
)

var (
	log = golog.LoggerFor("report")
)

type Handler struct {
	DB *DB
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
	table := strings.ToLower(req.URL.Path)[1:]
	if table == "" {
		badRequest(resp, "Missing table in path")
		return
	}

	query := req.URL.Query()
	resolution := 0 * time.Second
	resolutionString := query.Get("resolution")
	if resolutionString != "" {
		var parseErr error
		resolution, parseErr = time.ParseDuration(resolutionString)
		if parseErr != nil {
			badRequest(resp, "Error parsing resolution %v: %v", resolutionString, parseErr)
			return
		}
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

	fieldsString := query.Get("select")
	if fieldsString == "" {
		badRequest(resp, "Missing select in querystring")
		return
	}
	fields := make(map[string]expr.Expr, 0)
	var sortedFields []string
	for _, field := range strings.Split(fieldsString, ";") {
		parts := strings.Split(field, ":")
		if len(parts) != 2 {
			badRequest(resp, "select needs to be of the form field_a:Sum('a');field_b:Add(1, 'b')", fieldsString, err)
			return
		}
		e, parseErr := expr.JS(parts[1])
		if parseErr != nil {
			badRequest(resp, "Unable to parse expression %v for field %v: %v", parts[1], parts[0], parseErr)
			return
		}
		fields[parts[0]] = e
		sortedFields = append(sortedFields, parts[0])
	}

	groupBy := []string{}
	groupByString := query.Get("groupby")
	if groupByString != "" {
		groupBy = strings.Split(groupByString, ";")
	}

	orderBy := make(map[string]bool, 0)
	orderByString := query.Get("orderby")
	if orderByString != "" {
		for _, order := range strings.Split(orderByString, ";") {
			parts := strings.Split(order, ":")
			if len(parts) > 2 {
				badRequest(resp, "orderby needs to be of the form field_a:true;field_b;field_c:false", orderByString, err)
				return
			}
			if len(parts) == 1 {
				// Default to descending ordering
				orderBy[parts[0]] = false
				continue
			}
			asc, parseErr := strconv.ParseBool(parts[1])
			if parseErr != nil {
				badRequest(resp, "Unable to parse boolean %v: %v", parts[1], parseErr)
				return
			}
			orderBy[parts[0]] = asc
		}
	}

	aq := h.DB.Query(table, resolution).From(from).To(to)
	for field, e := range fields {
		aq.Select(field, e)
	}
	for _, dim := range groupBy {
		aq.GroupBy(dim)
	}
	for field, asc := range orderBy {
		aq.OrderBy(field, asc)
	}

	result, err := aq.Run()
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	fmt.Fprintf(resp, "# -------- %v --------\n", table)
	fmt.Fprintf(resp, "# From:       %v\n", result.From)
	fmt.Fprintf(resp, "# To:         %v\n", result.To)
	fmt.Fprintf(resp, "# Resolution: %v\n", result.Resolution)
	for _, field := range strings.Split(fieldsString, ";") {
		parts := strings.Split(field, ":")
		fmt.Fprintf(resp, "# Select:     %v -> %v\n", parts[0], parts[1])
	}
	fmt.Fprintf(resp, "# Group By:   %v\n", strings.Join(result.Dims, ";"))
	fmt.Fprintf(resp, "# Order By:   %v\n\n", orderByString)

	fmt.Fprintf(resp, "#%-20v", "time")
	for _, dim := range result.Dims {
		fmt.Fprintf(resp, "%-20v", dim)
	}

	for _, field := range sortedFields {
		fmt.Fprintf(resp, "%20v", field)
	}
	fmt.Fprint(resp, "\n")

	numPeriods := int(result.To.Sub(result.From) / result.Resolution)
	for _, entry := range result.Entries {
		for i := 0; i < numPeriods; i++ {
			fmt.Fprintf(resp, "%-20v", result.To.Add(-1*time.Duration(i)*result.Resolution))
			for _, dim := range result.Dims {
				fmt.Fprintf(resp, "%-20v", entry.Dims[dim])
			}
			for _, field := range sortedFields {
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
