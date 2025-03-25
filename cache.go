package niu

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	master *redis.Client
	slave  *redis.Client
}

// 初始化缓存
// slaveOpt 可以为空，此时slave与master共享同一实例
func NewCache(ctx context.Context, masterOpt, slaveOpt *redis.Options) (*Cache, error) {
	masterDb := redis.NewClient(masterOpt)
	_, err := masterDb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	c := &Cache{masterDb, masterDb}
	if slaveOpt != nil {
		slaveDb := redis.NewClient(slaveOpt)
		_, err := slaveDb.Ping(ctx).Result()
		if err != nil {
			return nil, err
		}
		c.slave = slaveDb
	}
	return c, nil
}

func (c *Cache) Master() *redis.Client {
	return c.master
}

func (c *Cache) Slave() *redis.Client {
	return c.slave
}

func (c *Cache) Close() error {
	if c.master != nil {
		client := c.master
		c.master = nil
		err := client.Close()
		if err != nil {
			return err
		}
	}
	if c.slave != nil {
		client := c.slave
		c.slave = nil
		err := client.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Cache) Batch(ctx context.Context, f func(pipe redis.Pipeliner)) (map[int]interface{}, map[int]error) {
	pp := c.master.TxPipeline()
	// defer pp.Discard()
	f(pp)

	cmders, err := pp.Exec(ctx)
	if err != nil {
		errMap := make(map[int]error, 1)
		errMap[0] = err
		return nil, errMap
	}
	if len(cmders) < 1 {
		errMap := make(map[int]error, 1)
		errMap[0] = errors.New("cmders must be greater than or equal to 1")
		return nil, errMap
	}

	return c.getCmdResult(cmders)
}

func (c *Cache) getCmdResult(cmders []redis.Cmder) (map[int]interface{}, map[int]error) {
	mapLen := len(cmders)
	if mapLen <= 0 {
		return nil, nil
	}

	strMap := make(map[int]interface{}, mapLen)
	errMap := make(map[int]error, mapLen)
	for idx, cmder := range cmders {
		mapIdx := idx

		//*ClusterSlotsCmd 未实现
		switch v := cmder.(type) {
		case *redis.Cmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.StringCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.SliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.StringSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.MapStringStringCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.KeyValueSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.MapStringIntCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.IntSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.BoolCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.BoolSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.IntCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.FloatCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.FloatSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.StatusCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.TimeCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.DurationCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.StringStructMapCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.XMessageSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.XStreamSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.XPendingCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.XPendingExtCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.ZSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.ZWithKeyCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.CommandsInfoCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.GeoLocationCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.GeoPosCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.MapStringInterfaceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		case *redis.MapStringStringSliceCmd:
			strMap[mapIdx], errMap[mapIdx] = v.Result()
		}
	}
	return strMap, errMap
}

func (c *Cache) KeyDel(ctx context.Context, keys ...string) (int64, error) {
	return c.master.Del(ctx, keys...).Result()
}

func (c *Cache) KeyExists(ctx context.Context, keys ...string) (int64, error) {
	return c.slave.Exists(ctx, keys...).Result()
}

func (c *Cache) KeyExpire(ctx context.Context, key string, expiry time.Duration) (bool, error) {
	return c.master.Expire(ctx, key, expiry).Result()
}

func (c *Cache) KeyExpireAt(ctx context.Context, key string, expiry time.Time) (bool, error) {
	return c.master.ExpireAt(ctx, key, expiry).Result()
}

func (c *Cache) DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	return c.master.DecrBy(ctx, key, decrement).Result()
}

func (c *Cache) IncrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	return c.master.IncrBy(ctx, key, decrement).Result()
}

func (c *Cache) IncrByFloat(ctx context.Context, key string, decrement float64) (float64, error) {
	return c.master.IncrByFloat(ctx, key, decrement).Result()
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	return c.slave.Get(ctx, key).Result()
}

func (c *Cache) GetJson(ctx context.Context, key string, out interface{}) error {
	jsonStr, err := c.slave.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonStr), out)
}

func (c *Cache) Set(ctx context.Context, key string, value any, expiry time.Duration) (string, error) {
	return c.master.Set(ctx, key, value, expiry).Result()
}

func (c *Cache) SetJson(ctx context.Context, key string, val any, expiry time.Duration) (string, error) {
	jsonStr, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return c.master.Set(ctx, key, string(jsonStr), expiry).Result()
}

func (c *Cache) SetNX(ctx context.Context, key string, value any, expiry time.Duration) (bool, error) {
	return c.master.SetNX(ctx, key, value, expiry).Result()
}

func (c *Cache) GetSet(ctx context.Context, key string, value any) (string, error) {
	return c.master.GetSet(ctx, key, value).Result()
}

func (c *Cache) GetExpire(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return c.master.GetEx(ctx, key, expiration).Result()
}

func (c *Cache) GetDel(ctx context.Context, key string) (string, error) {
	return c.master.GetDel(ctx, key).Result()
}

func (c *Cache) MultiGet(ctx context.Context, keys ...string) ([]any, error) {
	return c.slave.MGet(ctx, keys...).Result()
}

func (c *Cache) MultiSet(ctx context.Context, maps map[string]any) (string, error) {
	return c.master.MSet(ctx, maps).Result()
}

func (c *Cache) MultiSetNX(ctx context.Context, maps map[string]any) (bool, error) {
	return c.master.MSetNX(ctx, maps).Result()
}

func (c *Cache) HSetNX(ctx context.Context, key, field string, value any) (bool, error) {
	return c.master.HSetNX(ctx, key, field, value).Result()
}

func (c *Cache) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return c.master.HIncrBy(ctx, key, field, incr).Result()
}

func (c *Cache) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	return c.master.HIncrByFloat(ctx, key, field, incr).Result()
}

func (c *Cache) HGet(ctx context.Context, key, field string) (string, error) {
	return c.slave.HGet(ctx, key, field).Result()
}

func (c *Cache) HGetJson(ctx context.Context, key string, field string, out any) error {
	jsonStr, err := c.slave.HGet(ctx, key, field).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonStr), out)
}

func (c *Cache) HSet(ctx context.Context, key string, values map[string]any) (int64, error) {
	return c.master.HSet(ctx, key, values).Result()
}

func (c *Cache) HSetJson(ctx context.Context, key string, field string, val any) (int64, error) {
	jsonStr, err := json.Marshal(val)
	if err != nil {
		return -1, err
	}
	valMap := make(map[string]any)
	valMap[field] = string(jsonStr)
	return c.master.HSet(ctx, key, valMap).Result()
}

func (c *Cache) HDel(ctx context.Context, key string, fields ...string) (bool, error) {
	v, err := c.master.HDel(ctx, key, fields...).Result()
	if err != nil {
		return false, err
	}
	return v > 0, err
}

func (c *Cache) HExists(ctx context.Context, key, field string) (bool, error) {
	return c.master.HExists(ctx, key, field).Result()
}

func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.slave.HGetAll(ctx, key).Result()
}

func (c *Cache) HKeys(ctx context.Context, key string) ([]string, error) {
	return c.slave.HKeys(ctx, key).Result()
}

func (c *Cache) HLen(ctx context.Context, key string) (int64, error) {
	return c.slave.HLen(ctx, key).Result()
}

func (c *Cache) HMGet(ctx context.Context, key string, fields ...string) ([]any, error) {
	return c.slave.HMGet(ctx, key, fields...).Result()
}

func (c *Cache) HMSet(ctx context.Context, key string, values map[string]any) (bool, error) {
	return c.master.HMSet(ctx, key, values).Result()
}

func (c *Cache) HVals(ctx context.Context, key string) ([]string, error) {
	return c.slave.HVals(ctx, key).Result()
}

func (c *Cache) HMSetAndExpiry(ctx context.Context, key string, values map[string]string, expiry time.Duration) (bool, error) {
	_, err := c.Batch(ctx, func(pipe redis.Pipeliner) {
		pipe.HMSet(ctx, key, values)
		pipe.Expire(ctx, key, expiry)
	})

	for _, v := range err {
		if v != nil {
			return false, v
		}
	}

	return true, nil
}

func (c *Cache) SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	return c.master.SAdd(ctx, key, members...).Result()
}

func (c *Cache) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return c.slave.SIsMember(ctx, key, member).Result()
}

func (c *Cache) SMultiIsMember(ctx context.Context, key string, members ...any) ([]bool, error) {
	return c.slave.SMIsMember(ctx, key, members...).Result()
}

func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.slave.SMembers(ctx, key).Result()
}

func (c *Cache) SRemove(ctx context.Context, key string, members ...any) (int64, error) {
	return c.master.SRem(ctx, key, members...).Result()
}
