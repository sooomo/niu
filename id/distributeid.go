package id

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidDistributeIdParams = errors.New("invalid distribute id params")
)

type DistributeId struct {
	client        *redis.Client
	onceInitIdFac sync.Once
	Key           string
	Start         int
}

func NewDistributeId(ctx context.Context, client *redis.Client, key string, start int) (*DistributeId, error) {
	if client == nil || len(key) == 0 || start <= 0 {
		return nil, ErrInvalidDistributeIdParams
	}

	var d = DistributeId{
		client: client,
		Key:    key,
		Start:  start,
	}

	var err error
	d.onceInitIdFac.Do(func() {
		_, err = d.setNX(ctx, d.Key, d.Start, time.Duration(0))
	})

	return &d, err
}

func (c *DistributeId) Next(ctx context.Context) (int, error) {
	res, err := c.incrBy(ctx, c.Key, 1)
	if err != nil {
		return -1, err
	}
	return int(res), nil
}

func (c *DistributeId) setNX(ctx context.Context, key string, value any, expiry time.Duration) (bool, error) {
	res := c.client.SetNX(ctx, key, value, expiry)
	err := res.Err()
	if err != nil {
		return false, err
	}
	return res.Val(), err
}

func (c *DistributeId) incrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	res := c.client.IncrBy(ctx, key, decrement)
	err := res.Err()
	if err != nil {
		return -1, err
	}
	return res.Val(), err
}
