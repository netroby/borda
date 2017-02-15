package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/getlantern/borda"
	"github.com/getlantern/golog"
	gredis "github.com/getlantern/redis"
	"github.com/getlantern/zenodb/rpc/server"
	"github.com/getlantern/zenodb/web"
	"github.com/gorilla/mux"
	"github.com/vharitonsky/iniflags"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
	"gopkg.in/redis.v5"
)

var (
	log = golog.LoggerFor("borda")

	dbdir              = flag.String("dbdir", "zenodata", "The directory in which to place the database files, defaults to 'zenodata'")
	httpsaddr          = flag.String("httpsaddr", ":443", "The address at which to listen for HTTPS connections")
	cliaddr            = flag.String("cliaddr", "localhost:17712", "The address at which to listen for gRPC cli connections, defaults to localhost:17712")
	pprofAddr          = flag.String("pprofaddr", "localhost:4000", "if specified, will listen for pprof connections at the specified tcp address")
	cookieHashKey      = flag.String("cookiehashkey", "9Cb7CqeP3PVSQv7nq6B9XUaQmjXeA4RUHQctywefW7gu9fmc4wSPY7AVzhA9497H", "key to use for HMAC authentication of web auth cookies, should be 64 bytes, defaults to random 64 bytes if not specified")
	cookieBlockKey     = flag.String("cookieblockkey", "BtRxmTBveQUcX8ZYdfnrCN2mUB7z2juP", "key to use for encrypting web auth cookies, should be 32 bytes, defaults to random 32 bytes if not specified")
	oauthClientID      = flag.String("oauthclientid", "2780eb96d3834a26ebc2", "id to use for oauth client to connect to GitHub")
	oauthClientSecret  = flag.String("oauthclientsecret", "da57775e1c2d7956a50e81501491eabe48d45c14", "secret id to use for oauth client to connect to GitHub")
	gitHubOrg          = flag.String("githuborg", "getlantern", "the GitHug org against which web users are authenticated")
	ispdb              = flag.String("ispdb", "", "In order to enable ISP functions, point this to a maxmind ISP database file")
	aliasesFile        = flag.String("aliases", "aliases.props", "Optionally specify the path to a file containing expression aliases in the form alias=template(%v,%v) with one alias per line")
	sampleRate         = flag.Float64("samplerate", 0.2, "The sample rate (0.2 = 20%)")
	password           = flag.String("password", "GCKKjRHYxfeDaNhPmJnUs9cY3ewaHb", "The authentication token for accessing reports")
	maxWALSize         = flag.Int("maxwalsize", 1024*1024*1024, "Maximum size of WAL segments on disk. Defaults to 1 GB.")
	walCompressionSize = flag.Int("walcompressionsize", 30*1024*1024, "Size above which to start compressing WAL segments with snappy. Defaults to 30 MB.")
	numPartitions      = flag.Int("numpartitions", 1, "The number of partitions available to distribute amongst followers")
	redisAddr          = flag.String("redis", "", "Redis address in \"redis[s]://host:port\" format")
	redisCA            = flag.String("redisca", "", "Certificate for redislabs's CA")
	redisClientPK      = flag.String("redisclientpk", "", "Private key for authenticating client to redis's stunnel")
	redisClientCert    = flag.String("redisclientcert", "", "Certificate for authenticating client to redis's stunnel")
	redisCacheSize     = flag.Int("rediscachesize", 25000, "Configures the maximum size of redis caches for HGET operations, defaults to 25,000 per hash")
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

	s, db, err := borda.TDBSave(*dbdir, "schema.yaml", *aliasesFile, *ispdb, redisClient, *redisCacheSize, *maxWALSize, *walCompressionSize, *numPartitions)
	if err != nil {
		log.Fatalf("Unable to initialize tdb: %v", err)
	}

	m := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(_ context.Context, host string) error {
			// Support any host
			return nil
		},
		Cache: autocert.DirCache("certs"),
		Email: "admin@getlantern.org",
	}
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			origServerName := hello.ServerName
			// Always make it look like client requested a borda.getlantern.org cert
			hello.ServerName = "borda.getlantern.org"
			cert, certErr := m.GetCertificate(hello)
			hello.ServerName = origServerName
			return cert, certErr
		},
		PreferServerCipherSuites: true,
	}
	hl, err := tls.Listen("tcp", *httpsaddr, tlsConfig)
	if err != nil {
		log.Fatalf("Unable to listen HTTPS: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTPS connections at %v\n", hl.Addr())

	if *cliaddr != "" {
		cl, listenErr := tls.Listen("tcp", *cliaddr, tlsConfig)
		if listenErr != nil {
			log.Fatalf("Unable to listen at cliaddr %v: %v", *cliaddr, listenErr)
		}
		go rpcserver.Serve(db, cl, &rpcserver.Opts{
			Password: *password,
		})
	}

	log.Debugf("Sampling %f percent of inbound data", *sampleRate*100)

	h := &borda.Handler{Save: s, SampleRate: *sampleRate}
	go h.Report()
	router := mux.NewRouter()
	router.Handle("/measurements", h)
	err = web.Configure(db, router, &web.Opts{
		OAuthClientID:     *oauthClientID,
		OAuthClientSecret: *oauthClientSecret,
		GitHubOrg:         *gitHubOrg,
		HashKey:           *cookieHashKey,
		BlockKey:          *cookieBlockKey,
		Password:          *password,
	})
	if err != nil {
		panic(fmt.Errorf("Unable to configure web: %v", err))
	}
	hs := &http.Server{
		Handler:        router,
		ReadTimeout:    1 * time.Minute,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 19,
	}
	serverErr := hs.Serve(hl)
	if serverErr != nil {
		log.Fatalf("Error serving HTTPS: %v", serverErr)
	}
}
