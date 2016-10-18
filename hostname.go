package borda

import (
	"fmt"
	"github.com/getlantern/goexpr"
	"github.com/hashicorp/golang-lru"
	"gopkg.in/redis.v3"
	"time"
)

var (
	redisClient   *redis.Client
	hostnameCache *lru.Cache
)

func SetRedis(client *redis.Client) {
	redisClient = client
	hostnameCache, _ = lru.New(100000)
	go warmCache(client)
}

func warmCache(redisClient *redis.Client) {
	for {
		names, err := redisClient.HGetAllMap("srvip->server").Result()
		if err != nil {
			log.Errorf("Unable to get all servers from Redis: %v", err)
		} else {
			hostnameCache.Purge()
			for ip, name := range names {
				hostnameCache.Add(ip, name)
			}
		}
		time.Sleep(5 * time.Minute)
	}
}

func BuildHostname(ip goexpr.Expr) goexpr.Expr {
	return &hostnameExpr{ip}
}

type hostnameExpr struct {
	ip goexpr.Expr
}

func (e *hostnameExpr) Eval(params goexpr.Params) interface{} {
	ip := e.ip.Eval(params)
	if ip == nil {
		return nil
	}
	cached, found := hostnameCache.Get(ip)
	if found {
		return cached
	}
	ipString := ip.(string)
	name, _ := redisClient.HGet("srvip->server", ipString).Result()
	if name == "" {
		srv, _ := redisClient.HGet("srvip->srv", ipString).Result()
		if srv != "" {
			name, _ = redisClient.HGet("srv->name", srv).Result()
		}
	}
	if name == "" {
		name = ipString
	}
	hostnameCache.Add(ip, name)
	return name
}

func (e *hostnameExpr) String() string {
	return fmt.Sprintf("HOSTNAME(%v)", e.ip.String())
}
