package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getlantern/borda"
	"github.com/getlantern/golog"
	"github.com/getlantern/tlsdefaults"
	"github.com/vharitonsky/iniflags"
)

var (
	log = golog.LoggerFor("borda")

	httpsaddr     = flag.String("httpsaddr", ":62443", "The address at which to listen for HTTPS connections")
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

	if *influxpass == "" {
		log.Error("Please specify an influxpass")
		flag.Usage()
		os.Exit(1)
	}

	hl, err := tlsdefaults.Listen(*httpsaddr, *pkfile, *certfile)
	if err != nil {
		log.Fatalf("Unable to listen HTTPS: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTPS connections at %v\n", hl.Addr())

	s, err := borda.TDBSave("tdbdata")
	if err != nil {
		log.Fatalf("Unable to initialize tdb: %v", err)
	}

	h := &borda.Handler{s}
	serverErr := http.Serve(hl, h)
	if serverErr != nil {
		log.Fatalf("Error serving HTTPS: %v", serverErr)
	}
}
