package report

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getlantern/golog"
	"github.com/getlantern/zenodb"
)

const (
	XAuthToken = "X-Auth-Token"
)

var (
	log = golog.LoggerFor("report")
)

type Handler struct {
	DB        *zenodb.DB
	AuthToken string
}

// ServeHTTP implements the http.Handler interface and supports querying via
// HTTP.
func (h *Handler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(resp, "Method %v not allowed\n", req.Method)
		return
	}

	authToken := req.Header.Get(XAuthToken)
	if authToken == "" || authToken != h.AuthToken {
		log.Errorf("Missing or bad auth token: %v", authToken)
		resp.WriteHeader(http.StatusForbidden)
		return
	}
	resp.Header().Set("Content-Type", "text/csv")
	sql, err := url.QueryUnescape(req.URL.RawQuery)
	if err != nil {
		badRequest(resp, "Please url encode your sql query")
		return
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
		log.Error(err)
		resp.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(resp, "%v\n", err)
		return
	}

	porcelain := strings.EqualFold("/porcelain", req.URL.Path)

	w := csv.NewWriter(resp)
	if !porcelain {
		header := make([]string, 0, 1+len(result.GroupBy)+len(result.FieldNames))
		header = append(header, "time")
		for _, dim := range result.GroupBy {
			header = append(header, dim)
		}
		for _, field := range result.FieldNames {
			header = append(header, field)
		}
		writeErr := w.Write(header)
		if writeErr != nil {
			log.Error(writeErr)
			return
		}
	}

	i := 0
	for _, row := range result.Rows {
		line := make([]string, 0, 1+len(row.Dims)+len(row.Values))
		line = append(line, result.Until.Add(-1*time.Duration(row.Period)*result.Resolution).In(time.UTC).Format(time.RFC3339))
		for _, dim := range row.Dims {
			line = append(line, fmt.Sprint(nilToBlank(dim)))
		}
		for _, val := range row.Values {
			line = append(line, fmt.Sprintf("%f", val))
		}
		writeErr := w.Write(line)
		if writeErr != nil {
			log.Error(writeErr)
			return
		}
		i++
		if i%100 == 0 {
			w.Flush()
		}
	}

	w.Flush()
}

func badRequest(resp http.ResponseWriter, msg string, args ...interface{}) {
	log.Errorf(msg, args...)
	resp.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(resp, msg+"\n", args...)
}

func nilToBlank(val interface{}) interface{} {
	if val == nil {
		return ""
	}
	return val
}
