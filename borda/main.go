package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getlantern/borda"
	"github.com/getlantern/borda/report"
	"github.com/getlantern/golog"
	"github.com/getlantern/tlsdefaults"
	"github.com/vharitonsky/iniflags"
)

var (
	log = golog.LoggerFor("borda")

	httpsaddr     = flag.String("httpsaddr", ":62443", "The address at which to listen for HTTPS connections")
	reportsaddr   = flag.String("reportsaddr", ":14443", "The address at which to listen for HTTPS connections to the reports")
	pkfile        = flag.String("pkfile", "pk.pem", "Path to the private key PEM file")
	certfile      = flag.String("certfile", "cert.pem", "Path to the certificate PEM file")
	indexeddims   = flag.String("indexeddims", "app,client_ip,proxy_host", "Indexed Dimensions")
	influxurl     = flag.String("influxurl", "http://localhost:8086", "InfluxDB URL")
	influxdb      = flag.String("influxdb", "lantern2", "InfluxDB database name")
	influxuser    = flag.String("influxuser", "lantern2", "InfluxDB username")
	influxpass    = flag.String("influxpass", "", "InfluxDB password")
	batchsize     = flag.Int("batchsize", 100, "Batch size")
	batchwindow   = flag.Duration("batchwindow", 30*time.Second, "Batch window")
	maxretries    = flag.Int("maxretries", 25, "Maximum retries to write to InfluxDB before giving up")
	retryinterval = flag.Duration("retryinterval", 30*time.Second, "How long to wait between retries")
)

func main() {
	iniflags.Parse()

	s, db, err := borda.TDBSave("tdbdata")
	if err != nil {
		log.Fatalf("Unable to initialize tdb: %v", err)
	}

	rl, err := tlsdefaults.Listen(*httpsaddr, *pkfile, *certfile)
	if err != nil {
		log.Fatalf("Unable to listen for reports: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for report connections at %v\n", rl.Addr())
	r := &report.Handler{DB: db}
	go func() {
		serverErr := http.Serve(rl, r)
		if serverErr != nil {
			log.Errorf("Error serving reports: %v", serverErr)
		}
	}()

	hl, err := tlsdefaults.Listen(*httpsaddr, *pkfile, *certfile)
	if err != nil {
		log.Fatalf("Unable to listen HTTPS: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTPS connections at %v\n", hl.Addr())

	h := &borda.Handler{Save: s}
	serverErr := http.Serve(hl, h)
	if serverErr != nil {
		log.Fatalf("Error serving HTTPS: %v", serverErr)
	}
}
