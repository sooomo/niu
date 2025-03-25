package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var master *redis.Client
var slave *redis.Client

// 初始化缓存
// slaveOpt 可以为空，此时slave与master共享同一实例
func Init(ctx context.Context, masterOpt, slaveOpt *redis.Options) error {
	masterDb := redis.NewClient(masterOpt)
	_, err := masterDb.Ping(ctx).Result()
	if err != nil {
		return err
	}
	master = masterDb
	slave = masterDb
	if slaveOpt != nil {
		slaveDb := redis.NewClient(slaveOpt)
		_, err := slaveDb.Ping(ctx).Result()
		if err != nil {
			return err
		}
		slave = slaveDb
	}
	return nil
}

func Master() *redis.Client {
	return master
}

func Slave() *redis.Client {
	return slave
}

func Close() error {
	if master != nil {
		client := master
		master = nil
		err := client.Close()
		if err != nil {
			return err
		}
	}
	if slave != nil {
		client := slave
		slave = nil
		err := client.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func Batch(ctx context.Context, f func(pipe redis.Pipeliner)) (map[int]interface{}, map[int]error) {
	pp := master.TxPipeline()
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

	return getCmdResult(cmders)
}

func getCmdResult(cmders []redis.Cmder) (map[int]interface{}, map[int]error) {
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

func KeyDel(ctx context.Context, keys ...string) (int64, error) {
	return master.Del(ctx, keys...).Result()
}

func KeyExists(ctx context.Context, keys ...string) (int64, error) {
	return slave.Exists(ctx, keys...).Result()
}

func KeyExpire(ctx context.Context, key string, expiry time.Duration) (bool, error) {
	return master.Expire(ctx, key, expiry).Result()
}

func KeyExpireAt(ctx context.Context, key string, expiry time.Time) (bool, error) {
	return master.ExpireAt(ctx, key, expiry).Result()
}

func DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	return master.DecrBy(ctx, key, decrement).Result()
}

func IncrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	return master.IncrBy(ctx, key, decrement).Result()
}

func IncrByFloat(ctx context.Context, key string, decrement float64) (float64, error) {
	return master.IncrByFloat(ctx, key, decrement).Result()
}

func Get(ctx context.Context, key string) (string, error) {
	return slave.Get(ctx, key).Result()
}

func GetJson(ctx context.Context, key string, out interface{}) error {
	jsonStr, err := slave.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonStr), out)
}

func Set(ctx context.Context, key string, value any, expiry time.Duration) (string, error) {
	return master.Set(ctx, key, value, expiry).Result()
}

func SetJson(ctx context.Context, key string, val any, expiry time.Duration) (string, error) {
	jsonStr, err := json.Marshal(val)
	if err != nil {
		return "", err
	}
	return master.Set(ctx, key, string(jsonStr), expiry).Result()
}

func SetNX(ctx context.Context, key string, value any, expiry time.Duration) (bool, error) {
	return master.SetNX(ctx, key, value, expiry).Result()
}

func GetSet(ctx context.Context, key string, value any) (string, error) {
	return master.GetSet(ctx, key, value).Result()
}

func GetExpire(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return master.GetEx(ctx, key, expiration).Result()
}

func GetDel(ctx context.Context, key string) (string, error) {
	return master.GetDel(ctx, key).Result()
}

func MultiGet(ctx context.Context, keys ...string) ([]any, error) {
	return slave.MGet(ctx, keys...).Result()
}

func MultiSet(ctx context.Context, maps map[string]any) (string, error) {
	return master.MSet(ctx, maps).Result()
}

func MultiSetNX(ctx context.Context, maps map[string]any) (bool, error) {
	return master.MSetNX(ctx, maps).Result()
}

func HSetNX(ctx context.Context, key, field string, value any) (bool, error) {
	return master.HSetNX(ctx, key, field, value).Result()
}

func HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	return master.HIncrBy(ctx, key, field, incr).Result()
}

func HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	return master.HIncrByFloat(ctx, key, field, incr).Result()
}

func HGet(ctx context.Context, key, field string) (string, error) {
	return slave.HGet(ctx, key, field).Result()
}

func HGetJson(ctx context.Context, key string, field string, out any) error {
	jsonStr, err := slave.HGet(ctx, key, field).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(jsonStr), out)
}

func HSet(ctx context.Context, key string, values map[string]any) (int64, error) {
	return master.HSet(ctx, key, values).Result()
}

func HSetJson(ctx context.Context, key string, field string, val any) (int64, error) {
	jsonStr, err := json.Marshal(val)
	if err != nil {
		return -1, err
	}
	valMap := make(map[string]any)
	valMap[field] = string(jsonStr)
	return master.HSet(ctx, key, valMap).Result()
}

func HDel(ctx context.Context, key string, fields ...string) (bool, error) {
	v, err := master.HDel(ctx, key, fields...).Result()
	if err != nil {
		return false, err
	}
	return v > 0, err
}

func HExists(ctx context.Context, key, field string) (bool, error) {
	return master.HExists(ctx, key, field).Result()
}

func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return slave.HGetAll(ctx, key).Result()
}

func HKeys(ctx context.Context, key string) ([]string, error) {
	return slave.HKeys(ctx, key).Result()
}

func HLen(ctx context.Context, key string) (int64, error) {
	return slave.HLen(ctx, key).Result()
}

func HMGet(ctx context.Context, key string, fields ...string) ([]any, error) {
	return slave.HMGet(ctx, key, fields...).Result()
}

func HMSet(ctx context.Context, key string, values map[string]any) (bool, error) {
	return master.HMSet(ctx, key, values).Result()
}

func HVals(ctx context.Context, key string) ([]string, error) {
	return slave.HVals(ctx, key).Result()
}

func HMSetAndExpiry(ctx context.Context, key string, values map[string]string, expiry time.Duration) (bool, error) {
	_, err := Batch(ctx, func(pipe redis.Pipeliner) {
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

func SAdd(ctx context.Context, key string, members ...any) (int64, error) {
	return master.SAdd(ctx, key, members...).Result()
}

func SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return slave.SIsMember(ctx, key, member).Result()
}

func SMultiIsMember(ctx context.Context, key string, members ...any) ([]bool, error) {
	return slave.SMIsMember(ctx, key, members...).Result()
}

func SMembers(ctx context.Context, key string) ([]string, error) {
	return slave.SMembers(ctx, key).Result()
}

func SRemove(ctx context.Context, key string, members ...any) (int64, error) {
	return master.SRem(ctx, key, members...).Result()
}
