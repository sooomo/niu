package middlewares

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sooomo/niu/net"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func RateLimitMiddleware(limitInterval time.Duration, count int) gin.HandlerFunc {
	ipLimiter.init(limitInterval, count)
	return func(ctx *gin.Context) {
		ip := net.GetClientIp(ctx)
		if ip == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		if gin.IsDebugging() {
			fmt.Printf("[RateLimit] ip: %v", ip)
		}

		lm := ipLimiter.get(ip)
		if lm.Allow() {
			ctx.Next()
		} else {
			fmt.Printf("[RateLimit] not allowd, ip:%v", ip)
			ctx.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
	}
}

type ipRateLimiter struct {
	ips   map[string]*rate.Limiter
	mu    *sync.RWMutex
	limit rate.Limit
	b     int
}

var ipLimiter *ipRateLimiter = &ipRateLimiter{}

func (limiter *ipRateLimiter) init(limitInterval time.Duration, count int) {
	limiter.ips = make(map[string]*rate.Limiter)
	limiter.mu = &sync.RWMutex{}
	limiter.limit = rate.Every(limitInterval) // 每limitInterval往桶中放一个令牌，即每秒产生{1000/limitInterval}个令牌
	limiter.b = count                         // 令牌桶的容量：平均每秒最多有 count 个请求
}

func (limiter *ipRateLimiter) addIp(ip string) *rate.Limiter {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	l := rate.NewLimiter(limiter.limit, limiter.b)
	limiter.ips[ip] = l
	return l
}

func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.RLock()
	lm, exist := l.ips[ip]
	l.mu.RUnlock()
	if exist {
		return lm
	}

	return l.addIp(ip)
}
