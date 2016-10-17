package borda

import (
	"fmt"
	"github.com/getlantern/goexpr"
	"github.com/hashicorp/golang-lru"
	"gopkg.in/redis.v3"
)

var (
	redisClient *redis.Client
)

func SetRedis(client *redis.Client) {
	redisClient = client
}

func BuildHostname(ip goexpr.Expr) goexpr.Expr {
	cache, _ := lru.New(100000)
	return &hostnameExpr{ip, cache}
}

type hostnameExpr struct {
	ip    goexpr.Expr
	cache *lru.Cache
}

func (e *hostnameExpr) Eval(params goexpr.Params) interface{} {
	ip := e.ip.Eval(params)
	if ip == nil {
		return nil
	}
	cached, found := e.cache.Get(ip)
	if found {
		return cached
	}
	ipString := ip.(string)
	name := ""
	srv, _ := redisClient.HGet("srvip->srv", ipString).Result()
	if srv != "" {
		name, _ = redisClient.HGet("srv->name", srv).Result()
	}
	if name == "" {
		name = ipString
	}
	e.cache.Add(ip, name)
	return name
}

func (e *hostnameExpr) String() string {
	return fmt.Sprintf("HOSTNAME(%v)", e.ip.String())
}
