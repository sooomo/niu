package id

import (
	"fmt"
	"sync"
	"time"
)

const (
	epoch          = int64(1672502400)                 // 设置起始时间(时间戳/秒)：2023-01-01 00:00:00，有效期69年
	timestampBits  = uint(50)                          // 时间戳占用位数
	workeridBits   = uint(3)                           // 机器id所占位数
	sequenceBits   = uint(10)                          // 序列所占的位数
	timestampMax   = int64(-1 ^ (-1 << timestampBits)) // 时间戳最大值
	workeridMax    = int64(-1 ^ (-1 << workeridBits))  // 支持的最大机器id数量
	sequenceMask   = int64(-1 ^ (-1 << sequenceBits))  // 支持的最大序列id数量
	workeridShift  = sequenceBits                      // 机器id左移位数
	timestampShift = sequenceBits + workeridBits       // 时间戳左移位数
)

type Snowflake struct {
	sync.Mutex
	timestamp int64
	workerid  int64
	sequence  int64
}

func NewSnowflake(workerid int64) *Snowflake {
	if workerid < 0 || workerid > workeridMax {
		panic(fmt.Sprintf("workerid must be between 0 and %d", workeridMax-1))
	}
	return &Snowflake{
		timestamp: 0,
		workerid:  workerid,
		sequence:  0,
	}
}

func (s *Snowflake) Next() int64 {
	s.Lock()
	now := time.Now().Unix()
	if s.timestamp == now {
		// 当同一时间戳（精度：秒）下多次生成id会增加序列号
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			// 如果当前序列超出12bit长度，则需要等待下一秒
			// 下一秒将使用sequence:0
			for now <= s.timestamp {
				now = time.Now().Unix()
			}
		}
	} else {
		// 不同时间戳（精度：秒）下直接使用序列号：0
		s.sequence = 0
	}
	t := now - epoch
	if t > timestampMax {
		s.Unlock()
		return 0
	}
	s.timestamp = now
	r := int64((t)<<timestampShift | (s.workerid << workeridShift) | (s.sequence))
	s.Unlock()
	return r
}

// 获取机器ID
func GetWorkerId(sid int64) int64 {
	return (sid >> workeridShift) & workeridMax
}

// 获取时间戳
func GetTimestamp(sid int64) int64 {
	return (sid >> timestampShift) & timestampMax
}

// 获取创建ID时的时间戳
func GetGenTimestamp(sid int64) int64 {
	return GetTimestamp(sid) + epoch
}

// 获取创建ID时的时间字符串(精度：秒)
func GetGenTime(sid int64) string {
	// 需将GetGenTimestamp获取的时间戳
	return time.Unix(GetGenTimestamp(sid), 0).Format("2006-01-02 15:04:05")
}

// 获取时间戳已使用的占比：范围（0.0 - 1.0）
func GetTimestampStatus() float64 {
	return float64(time.Now().Unix()/-epoch) / float64(timestampMax)
}
