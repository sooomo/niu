package cachetest_test

import (
	"context"
	"niu/cache"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestLinkQueue_In(t *testing.T) {
	ctx := context.Background()
	err := cache.Init(ctx, &redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	}, nil)
	if err != nil {
		t.Error(err)
		return
	}
	ret, err := cache.Set(ctx, "go_test", time.Now(), time.Minute*5)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(ret)
}
