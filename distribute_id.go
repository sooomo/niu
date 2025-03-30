package niu

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
	sync.RWMutex
	client       *redis.Client
	idGenerators map[string]*IdGenerator
}

func NewDistributeId(ctx context.Context, opt *redis.Options) (*DistributeId, error) {
	client := redis.NewClient(opt)
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return &DistributeId{client: client, idGenerators: make(map[string]*IdGenerator)}, nil
}

func (d *DistributeId) NewGenerator(ctx context.Context, key string, start int) (*IdGenerator, error) {
	d.Lock()
	defer d.Unlock()
	idGen, ok := d.idGenerators[key]
	if ok {
		return idGen, nil
	}
	idGen = &IdGenerator{
		onceInitIdFac: sync.Once{},
		client:        d.client,
		key:           key,
		start:         start,
	}
	err := idGen.init(ctx)
	if err != nil {
		return nil, err
	}

	d.idGenerators[key] = idGen
	return idGen, nil
}

func (d *DistributeId) Close() {
	d.Lock()
	defer d.Unlock()
	d.client.Close()
	clear(d.idGenerators)
}

type IdGenerator struct {
	client        *redis.Client
	onceInitIdFac sync.Once
	key           string
	start         int
}

func (d *IdGenerator) init(ctx context.Context) error {
	var err error
	d.onceInitIdFac.Do(func() {
		_, err = d.client.SetNX(ctx, d.key, d.start, time.Duration(0)).Result()
	})

	return err
}

func (c *IdGenerator) Next(ctx context.Context) (int, error) {
	res, err := c.client.IncrBy(ctx, c.key, 1).Result()
	if err != nil {
		return -1, err
	}
	return int(res), nil
}
