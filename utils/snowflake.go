package utils

import (
	"fmt"
	"sync"
	"time"
)

// Snowflake 雪花算法生成唯一ID
// 用于生成全局唯一的消息ID
type Snowflake struct {
	mu            sync.Mutex
	epoch         int64 // 起始时间戳（毫秒）
	machineID     int64 // 机器ID
	datacenterID  int64 // 数据中心ID
	sequence      int64 // 序列号
	lastTimestamp int64 // 上次生成ID的时间戳
}

// NewSnowflake 创建雪花算法实例
// machineID: 机器ID (0-1023)
// datacenterID: 数据中心ID (0-31)
func NewSnowflake(machineID, datacenterID int64) *Snowflake {
	return &Snowflake{
		epoch:        1609459200000,       // 2021-01-01 00:00:00 UTC
		machineID:    machineID & 0x3FF,   // 10位
		datacenterID: datacenterID & 0x1F, // 5位
	}
}

// NextID 生成下一个ID
// 64位ID结构：1位符号位(0) + 41位时间戳 + 5位数据中心ID + 10位机器ID + 12位序列号
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixNano() / 1e6 // 当前时间戳（毫秒）

	if now < s.lastTimestamp {
		// 时钟回拨，等待
		time.Sleep(time.Duration(s.lastTimestamp-now) * time.Millisecond)
		now = time.Now().UnixNano() / 1e6
	}

	if now == s.lastTimestamp {
		// 同一毫秒内，序列号递增
		s.sequence = (s.sequence + 1) & 0xFFF // 12位序列号
		if s.sequence == 0 {
			// 序列号溢出，等待下一毫秒
			now = s.waitNextMillis(s.lastTimestamp)
		}
	} else {
		// 新的毫秒，序列号重置
		s.sequence = 0
	}

	s.lastTimestamp = now

	// 生成ID
	id := ((now - s.epoch) << 22) | // 41位时间戳
		(s.datacenterID << 17) | // 5位数据中心ID
		(s.machineID << 12) | // 10位机器ID
		s.sequence // 12位序列号

	return id
}

// waitNextMillis 等待下一毫秒
func (s *Snowflake) waitNextMillis(lastTimestamp int64) int64 {
	now := time.Now().UnixNano() / 1e6
	for now <= lastTimestamp {
		now = time.Now().UnixNano() / 1e6
	}
	return now
}

// GenerateMessageID 生成消息ID字符串
func (s *Snowflake) GenerateMessageID() string {
	return fmt.Sprintf("%d", s.NextID())
}
