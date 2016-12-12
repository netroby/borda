package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/getlantern/borda"
	"github.com/getlantern/golog"
	gredis "github.com/getlantern/redis"
	"github.com/getlantern/tlsdefaults"
	"github.com/getlantern/zenodb/rpc"
	"github.com/vharitonsky/iniflags"
	"gopkg.in/redis.v3"
)

var (
	log = golog.LoggerFor("borda")

	httpsaddr         = flag.String("httpsaddr", ":443", "The address at which to listen for HTTPS connections")
	cliaddr           = flag.String("cliaddr", "localhost:17712", "The address at which to listen for gRPC cli connections, defaults to localhost:17712")
	pprofAddr         = flag.String("pprofaddr", "localhost:4000", "if specified, will listen for pprof connections at the specified tcp address")
	pkfile            = flag.String("pkfile", "pk.pem", "Path to the private key PEM file")
	certfile          = flag.String("certfile", "cert.pem", "Path to the certificate PEM file")
	ispdb             = flag.String("ispdb", "", "In order to enable ISP functions, point this to a maxmind ISP database file")
	aliasesFile       = flag.String("aliases", "aliases.props", "Optionally specify the path to a file containing expression aliases in the form alias=template(%v,%v) with one alias per line")
	sampleRate        = flag.Float64("samplerate", 0.2, "The sample rate (0.2 = 20%)")
	password          = flag.String("password", "GCKKjRHYxfeDaNhPmJnUs9cY3ewaHb", "The authentication token for accessing reports")
	maxWALAge         = flag.Duration("maxwalage", 336*time.Hour, "Maximum age for WAL files. Files older than this will be deleted. Defaults to 336 hours (2 weeks)")
	walCompressionAge = flag.Duration("walcompressage", 1*time.Hour, "Age at which to start compressing WAL files with gzip. Defaults to 1 hour.")
	numPartitions     = flag.Int("numpartitions", 1, "The number of partitions available to distribute amongst followers")
	redisAddr         = flag.String("redis", "", "Redis address in \"redis[s]://host:port\" format")
	redisCA           = flag.String("redisca", "", "Certificate for redislabs's CA")
	redisClientPK     = flag.String("redisclientpk", "", "Private key for authenticating client to redis's stunnel")
	redisClientCert   = flag.String("redisclientcert", "", "Certificate for authenticating client to redis's stunnel")
	redisCacheSize    = flag.Int("rediscachesize", 25000, "Configures the maximum size of redis caches for HGET operations, defaults to 25,000 per hash")
)

func main() {
	iniflags.Parse()

	if *pprofAddr != "" {
		go func() {
			log.Debugf("Starting pprof page at http://%s/debug/pprof", *pprofAddr)
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				log.Error(err)
			}
		}()
	}

	var redisClient *redis.Client
	if *redisAddr != "" {
		var redisErr error
		redisClient, redisErr = gredis.NewClient(&gredis.Opts{
			RedisURL:       *redisAddr,
			RedisCAFile:    *redisCA,
			ClientPKFile:   *redisClientPK,
			ClientCertFile: *redisClientCert,
		})
		if redisErr == nil {
			log.Debugf("Connected to Redis at %v", *redisAddr)
		} else {
			log.Errorf("Unable to connect to redis: %v", redisErr)
		}
	}

	s, db, err := borda.TDBSave("zenodata", "schema.yaml", *aliasesFile, *ispdb, redisClient, *redisCacheSize, *maxWALAge, *walCompressionAge, *numPartitions)
	if err != nil {
		log.Fatalf("Unable to initialize tdb: %v", err)
	}

	hl, err := tlsdefaults.Listen(*httpsaddr, *pkfile, *certfile)
	if err != nil {
		log.Fatalf("Unable to listen HTTPS: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTPS connections at %v\n", hl.Addr())

	if *cliaddr != "" {
		cl, err := tlsdefaults.Listen(*cliaddr, *pkfile, *certfile)
		if err != nil {
			log.Fatalf("Unable to listen at cliaddr %v: %v", *cliaddr, err)
		}
		go rpc.Serve(db, cl, &rpc.ServerOpts{
			Password: *password,
		})
	}

	log.Debugf("Sampling %f percent of inbound data", *sampleRate*100)

	h := &borda.Handler{Save: s, SampleRate: *sampleRate}
	go h.Report()
	serverErr := http.Serve(hl, h)
	if serverErr != nil {
		log.Fatalf("Error serving HTTPS: %v", serverErr)
	}
}
