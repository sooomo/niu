package niu

import (
	"context"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// id 消费者需要通过此Id来判断该消息是否已被消费
type ConsumeMsgHandler func(ctx context.Context, id string, msg map[string]any) error

type MessageQueue interface {
	Publish(ctx context.Context, topic string, body map[string]any) error
	Subscribe(ctx context.Context, topic, group, consumer string, handler ConsumeMsgHandler) error
}

type RedisMessageQueue struct {
	mutex      sync.RWMutex
	client     *redis.Client // Redis连接
	pool       CoroutinePool // 协程池
	xaddMaxLen int           // 发布消息时XAddArgs中MaxLen的值
	batchSize  int           // 消费消息时每次批量获取一批的大小
	closeChan  chan Empty
}

func NewRedisMessageQueue(ctx context.Context, opt *redis.Options, pool CoroutinePool, xaddMaxLen, batchSize int) (*RedisMessageQueue, error) {
	client := redis.NewClient(opt)
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return &RedisMessageQueue{sync.RWMutex{}, client, pool, xaddMaxLen, batchSize, make(chan Empty)}, nil
}

func (m *RedisMessageQueue) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.client == nil {
		return
	}
	m.closeChan <- Empty{}
	m.client.Close()
	m.client = nil
}

// 发布消息
func (m *RedisMessageQueue) Publish(ctx context.Context, topic string, body map[string]any) error {
	res := m.client.XAdd(ctx, &redis.XAddArgs{
		Stream: topic,
		MaxLen: int64(m.xaddMaxLen),
		Approx: true,
		ID:     "*", // 让Redis生成时间戳和序列号
		Values: body,
	})
	return res.Err()
}

// 开启协程后台消费。返回值代表消费过程中遇到的无法处理的错误
// group 消费者组，一般为当前服务的名称
// consumer 消费者组里的消费者，一般为一个uuid
// handler 消费消息的处理器，如果返回nil，则表示消息被成功消费，如果返回非nil，则表示消息被消费失败，需要重试
func (m *RedisMessageQueue) Subscribe(ctx context.Context, topic, group, consumer string, handler ConsumeMsgHandler) error {
	res := m.client.XGroupCreateMkStream(ctx, topic, group, "0") // start 用于创建消费者组的时候指定起始消费ID，0表示从头开始消费，$表示从最后一条消息开始消费
	err := res.Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return err
	}
	return m.pool.Submit(func() {
		for {
			select {
			case <-m.closeChan:
				return
			case <-ctx.Done():
				return
			default:
				// 拉取新消息
				if err := m.consume(ctx, topic, group, consumer, ">", m.batchSize, handler); err != nil {
					continue
				}
				// 拉取已经投递却未被ACK的消息，保证消息至少被成功消费1次
				if err := m.consume(ctx, topic, group, consumer, "0", m.batchSize, handler); err != nil {
					continue
				}
			}
		}
	})
}

func (m *RedisMessageQueue) consume(ctx context.Context, topic, group, consumer, id string, batchSize int, h ConsumeMsgHandler) error {
	// 阻塞的获取消息
	result, err := m.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{topic, id},
		Count:    int64(batchSize),
		NoAck:    false,
	}).Result()
	if err != nil {
		return err
	}
	// 处理消息
	for _, msg := range result[0].Messages {
		select {
		case <-m.closeChan:
			return nil
		case <-ctx.Done():
			return nil
		default:
			err := h(ctx, msg.ID, msg.Values)
			if err != nil {
				continue
			}
			err = m.client.XAck(ctx, topic, group, msg.ID).Err()
			if err != nil {
				continue
			}
		}
	}
	return nil
}
