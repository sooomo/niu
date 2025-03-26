package niu

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrLockFailed  = errors.New("lock failed")
	ErrLockNotHeld = errors.New("lock not held")
)

var (
	luaRefresh = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pexpire", KEYS[1], ARGV[2]) else return 0 end`)
	luaRelease = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)
	// luaPTTL    = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("pttl", KEYS[1]) else return -3 end`)
)

type DistributeLockOptions struct {
	Resource      string
	Token         string
	Ttl           time.Duration
	RetryStrategy RetryStrategy
}

type DistributeLocker struct {
	redisClient          *redis.Client
	defaultTtl           time.Duration
	defaultRetryStrategy RetryStrategy
}

func NewDistributeLocker(client *redis.Client, ttl time.Duration, retryStrategy RetryStrategy) *DistributeLocker {
	return &DistributeLocker{client, ttl, retryStrategy}
}

func NewDistributeLockerWithAddr(ctx context.Context, addr string, ttl time.Duration, retryStrategy RetryStrategy) (*DistributeLocker, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return &DistributeLocker{client, ttl, retryStrategy}, nil
}

func (l *DistributeLocker) Lock(ctx context.Context, resource string, token string) (*DistributeLock, error) {
	return l.LockWithOptions(ctx, &DistributeLockOptions{
		Resource:      resource,
		Token:         token,
		Ttl:           l.defaultTtl,
		RetryStrategy: l.defaultRetryStrategy,
	})
}

// 在指定资源上加锁，默认5s
func (l *DistributeLocker) LockWithOptions(ctx context.Context, opt *DistributeLockOptions) (*DistributeLock, error) {
	ttl := l.defaultTtl

	if opt.Ttl > 0 {
		ttl = opt.Ttl
	}
	retryStrategy := l.defaultRetryStrategy
	if opt.RetryStrategy != nil {
		retryStrategy = opt.RetryStrategy
	}
	// make sure we don't retry forever
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(ttl))
		defer cancel()
	}

	for {
		ok, err := l.redisClient.SetNX(ctx, opt.Resource, opt.Token, ttl).Result()
		if err != nil {
			return nil, err
		} else if ok {
			return &DistributeLock{l.redisClient, opt.Resource, opt.Token}, nil
		}
		// time.Sleep(1 * time.Second) // mock lock process

		// retry
		backoff := retryStrategy.Next()
		if backoff <= time.Duration(0) {
			return nil, ErrLockFailed
		}
		delay := time.After(backoff)

		select {
		case <-ctx.Done():
			return nil, ErrLockFailed
		case <-delay:
		}
	}
}

type DistributeLock struct {
	client   *redis.Client
	resource string
	token    string
}

func (i *DistributeLock) Refresh(ctx context.Context, ttl time.Duration) error {
	if i == nil {
		return nil
	}
	ttlVal := strconv.FormatInt(int64(ttl/time.Millisecond), 10)
	status, err := luaRefresh.Run(ctx, i.client, []string{i.resource}, i.token, ttlVal).Result()
	if err != nil {
		return err
	} else if status == int64(1) {
		return nil
	}
	return ErrLockNotHeld
}

// 释放获取的锁
func (i *DistributeLock) Release(ctx context.Context) error {
	if i == nil {
		return nil
	}
	res, err := luaRelease.Run(ctx, i.client, []string{i.resource}, i.token).Result()
	if err == redis.Nil {
		return ErrLockNotHeld
	} else if err != nil {
		return err
	}

	if i, ok := res.(int64); !ok || i != 1 {
		return ErrLockNotHeld
	}
	return nil
}
