package niu

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

// id 消费者需要通过此Id来判断该消息是否已被消费
type ConsumeMsgHandler func(id string, msg *map[string]any) error

type MessageQueue interface {
	Publish(ctx context.Context, topic string, body map[string]any) error
	Consume(ctx context.Context, topic, group, consumer string, handler ConsumeMsgHandler) error
}

type RedisMessageQueue struct {
	client     *redis.Client // Redis连接
	pool       RoutinePool   // 协程池
	xaddMaxLen int           // 发布消息时XAddArgs中MaxLen的值
	batchSize  int           // 消费消息时每次批量获取一批的大小
}

func NewRedisMessageQueue(c *redis.Client, pool RoutinePool, xaddMaxLen, batchSize int) *RedisMessageQueue {
	return &RedisMessageQueue{c, pool, xaddMaxLen, batchSize}
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
func (m *RedisMessageQueue) Consume(ctx context.Context, topic, group, consumer string, handler ConsumeMsgHandler) error {
	res := m.client.XGroupCreateMkStream(ctx, topic, group, "0") // start 用于创建消费者组的时候指定起始消费ID，0表示从头开始消费，$表示从最后一条消息开始消费
	err := res.Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return err
	}
	return m.pool.Submit(func() {
		for {
			// 拉取新消息
			if err := m.consume(ctx, topic, group, consumer, ">", m.batchSize, handler); err != nil {
				return
			}
			// 拉取已经投递却未被ACK的消息，保证消息至少被成功消费1次
			if err := m.consume(ctx, topic, group, consumer, "0", m.batchSize, handler); err != nil {
				return
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
		err := h(msg.ID, &msg.Values)
		if err == nil {
			err := m.client.XAck(ctx, topic, group, msg.ID).Err()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
