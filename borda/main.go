package main

import (
	"crypto/rand"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/getlantern/borda"
	"github.com/getlantern/golog"
	"github.com/getlantern/tlsredis"
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

	dbdir                    = flag.String("dbdir", "zenodata", "The directory in which to place the database files, defaults to 'zenodata'")
	httpaddr                 = flag.String("httpaddr", ":80", "The address at which to listen for HTTP connections (only used for LetsEncrypt http-01 challenges)")
	httpsaddr                = flag.String("httpsaddr", ":443", "The address at which to listen for HTTPS connections")
	cliaddr                  = flag.String("cliaddr", "localhost:17712", "The address at which to listen for gRPC cli connections, defaults to localhost:17712")
	pprofAddr                = flag.String("pprofaddr", "localhost:4000", "if specified, will listen for pprof connections at the specified tcp address")
	cookieHashKey            = flag.String("cookiehashkey", "9Cb7CqeP3PVSQv7nq6B9XUaQmjXeA4RUHQctywefW7gu9fmc4wSPY7AVzhA9497H", "key to use for HMAC authentication of web auth cookies, should be 64 bytes, defaults to random 64 bytes if not specified")
	cookieBlockKey           = flag.String("cookieblockkey", "BtRxmTBveQUcX8ZYdfnrCN2mUB7z2juP", "key to use for encrypting web auth cookies, should be 32 bytes, defaults to random 32 bytes if not specified")
	oauthClientID            = flag.String("oauthclientid", "2780eb96d3834a26ebc2", "id to use for oauth client to connect to GitHub")
	oauthClientSecret        = flag.String("oauthclientsecret", "da57775e1c2d7956a50e81501491eabe48d45c14", "secret id to use for oauth client to connect to GitHub")
	gitHubOrg                = flag.String("githuborg", "getlantern", "the GitHug org against which web users are authenticated")
	ispdb                    = flag.String("ispdb", "", "In order to enable ISP functions, point this to a maxmind ISP database file")
	aliasesFile              = flag.String("aliases", "aliases.props", "Optionally specify the path to a file containing expression aliases in the form alias=template(%v,%v) with one alias per line")
	sampleRate               = flag.Float64("samplerate", 0.2, "The sample rate (0.2 = 20%)")
	password                 = flag.String("password", "GCKKjRHYxfeDaNhPmJnUs9cY3ewaHb", "The authentication token for accessing reports")
	maxWALSize               = flag.Int("maxwalsize", 1024*1024*1024, "Maximum size of WAL segments on disk. Defaults to 1 GB.")
	walCompressionSize       = flag.Int("walcompressionsize", 30*1024*1024, "Size above which to start compressing WAL segments with snappy. Defaults to 30 MB.")
	maxMemory                = flag.Float64("maxmemory", 0.7, "Set to a non-zero value to cap the total size of the process as a percentage of total system memory. Defaults to 0.7 = 70%.")
	numPartitions            = flag.Int("numpartitions", 1, "The number of partitions available to distribute amongst followers")
	clusterQueryConcurrency  = flag.Int("clusterqueryconcurrency", 100, "specifies the maximum concurrency for clustered queries")
	redisAddr                = flag.String("redis", "", "Redis address in \"redis[s]://host:port\" format")
	redisCA                  = flag.String("redisca", "", "Certificate for redislabs's CA")
	redisClientPK            = flag.String("redisclientpk", "", "Private key for authenticating client to redis's stunnel")
	redisClientCert          = flag.String("redisclientcert", "", "Certificate for authenticating client to redis's stunnel")
	redisCacheSize           = flag.Int("rediscachesize", 50000, "Configures the maximum size of redis caches for HGET operations, defaults to 50,000 per hash")
	webQueryCacheTTL         = flag.Duration("webquerycachettl", 2*time.Hour, "specifies how long to cache web query results")
	webQueryTimeout          = flag.Duration("webquerytimeout", 30*time.Minute, "time out web queries after this duration")
	webQueryConcurrencyLimit = flag.Int("webqueryconcurrency", 2, "limit concurrent web queries to this (subsequent queries will be queued)")
	webMaxResponseBytes      = flag.Int("webquerymaxresponsebytes", 25*1024*1024, "limit the size of query results returned through the web API")
	httpsServerName          = flag.String("httpsservername", "borda.lantern.io", "the server name used for https connections")
	grpcServerName           = flag.String("grpcservername", "borda.getlantern.org", "the server name used for grpc connection")
	domainFrontedServerName  = flag.String("domainfrontedservername", "d157vud77ygy87.cloudfront.net", "the server name used for domain-fronted connections")
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
		redisClient, redisErr = tlsredis.GetClient(&tlsredis.Options{
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

	s, db, err := borda.TDBSave(*dbdir, "schema.yaml", *aliasesFile, *ispdb, redisClient, *redisCacheSize, *maxWALSize, *walCompressionSize, *numPartitions, *clusterQueryConcurrency, *maxMemory)
	if err != nil {
		log.Fatalf("Unable to initialize tdb: %v", err)
	}

	m := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		HostPolicy: func(_ context.Context, host string) error {
			// Support any host
			return nil
		},
		Cache:    autocert.DirCache("certs"),
		Email:    "admin@getlantern.org",
		ForceRSA: true, // we need to force RSA keys because CloudFront doesn't like our ECDSA cipher suites
	}
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			origServerName := hello.ServerName
			if origServerName == *domainFrontedServerName || origServerName == *httpsServerName || origServerName == "" {
				// Return the grpcServerName cert for domain-fronted requests, requests
				// the the httpsServerName (from CDN) or requests without SNI.
				hello.ServerName = *grpcServerName
			} else if origServerName != *grpcServerName {
			}
			cert, certErr := m.GetCertificate(hello)
			hello.ServerName = origServerName
			return cert, certErr
		},
		PreferServerCipherSuites: true,
		SessionTicketKey:         getSessionTicketKey(),
	}
	hls, err := tls.Listen("tcp", *httpsaddr, tlsConfig)
	if err != nil {
		log.Fatalf("Unable to listen HTTPS: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTPS connections at %v\n", hls.Addr())

	hl, err := net.Listen("tcp", *httpaddr)
	if err != nil {
		log.Fatalf("Unable to listen HTTP: %v", err)
	}
	fmt.Fprintf(os.Stdout, "Listening for HTTP connections at %v\n", hl.Addr())

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
	router.Use(borda.ForceCDN(*httpsServerName))
	router.Handle("/measurements", http.HandlerFunc(h.Measurements))
	router.Handle("/ping", http.HandlerFunc(h.Ping))
	err = web.Configure(db, router, &web.Opts{
		OAuthClientID:         *oauthClientID,
		OAuthClientSecret:     *oauthClientSecret,
		GitHubOrg:             *gitHubOrg,
		HashKey:               *cookieHashKey,
		BlockKey:              *cookieBlockKey,
		Password:              *password,
		CacheDir:              filepath.Join(*dbdir, "_webcache"),
		CacheTTL:              *webQueryCacheTTL,
		QueryTimeout:          *webQueryTimeout,
		QueryConcurrencyLimit: *webQueryConcurrencyLimit,
		MaxResponseBytes:      *webMaxResponseBytes,
	})
	if err != nil {
		panic(fmt.Errorf("Unable to configure web: %v", err))
	}
	hs := &http.Server{
		Handler:        m.HTTPHandler(nil),
		MaxHeaderBytes: 1 << 19,
	}
	go func() {
		err := hs.Serve(hl)
		if err != nil {
			log.Fatalf("Unable to serve HTTP traffic: %v", err)
		}
	}()
	hss := &http.Server{
		Handler:        router,
		MaxHeaderBytes: 1 << 19,
	}
	serverErr := hss.Serve(hls)
	if serverErr != nil {
		log.Fatalf("Error serving HTTPS: %v", serverErr)
	}
}

// this allows us to reuse a session ticket key across restarts, which avoids
// excessive TLS renegotiation with old clients.
func getSessionTicketKey() [32]byte {
	var key [32]byte
	keySlice, err := ioutil.ReadFile("session_ticket_key")
	if err != nil {
		keySlice = make([]byte, 32)
		n, err := rand.Read(keySlice)
		if err != nil {
			log.Errorf("Unable to generate session ticket key: %v", err)
			return key
		}
		if n != 32 {
			log.Errorf("Generated unexpected length of random data %d", n)
			return key
		}
		err = ioutil.WriteFile("session_ticket_key", keySlice, 0600)
		if err != nil {
			log.Errorf("Unable to save session_ticket_key: %v", err)
		} else {
			log.Debug("Saved new session_ticket_key")
		}
	}
	copy(key[:], keySlice)
	return key
}
