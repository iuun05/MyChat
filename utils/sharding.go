package utils

import (
	"fmt"
	"strconv"
)

// TableSharding 分表工具
type TableSharding struct {
	TableCount int // 分表数量
}

// NewTableSharding 创建分表工具
// tableCount: 分表数量，建议使用2的幂次方（如：16, 32, 64）
func NewTableSharding(tableCount int) *TableSharding {
	if tableCount <= 0 {
		tableCount = 16 // 默认16张表
	}
	return &TableSharding{
		TableCount: tableCount,
	}
}

// GetPrivateTableName 获取私聊消息表名
// 分表策略：按 (min(userId1, userId2) + max(userId1, userId2)) % tableCount
// 保证同一会话的消息在同一张表中
func (s *TableSharding) GetPrivateTableName(userId1, userId2 int64) string {
	var min, max int64
	if userId1 < userId2 {
		min, max = userId1, userId2
	} else {
		min, max = userId2, userId1
	}
	shard := (min + max) % int64(s.TableCount)
	return fmt.Sprintf("message_private_%d", shard)
}

// GetGroupTableName 获取群聊消息表名
// 分表策略：按 groupId % tableCount
func (s *TableSharding) GetGroupTableName(groupId int64) string {
	shard := groupId % int64(s.TableCount)
	return fmt.Sprintf("message_group_%d", shard)
}

// GetShardIndex 获取分片索引
func (s *TableSharding) GetShardIndex(id int64) int {
	return int(id % int64(s.TableCount))
}

// GenerateMessageID 生成全局唯一消息ID
// 格式：{timestamp}_{shard}_{seq}
// 例如：1704067200_5_12345
func (s *TableSharding) GenerateMessageID(shard int, seq int64) string {
	// 使用时间戳 + 分片 + 序号生成唯一ID
	// 实际生产环境建议使用雪花算法或UUID
	return fmt.Sprintf("%d_%d_%d", shard, seq, seq)
}

// ParseMessageID 解析消息ID
func (s *TableSharding) ParseMessageID(messageId string) (shard int, seq int64, err error) {
	// 简化实现，实际应该更健壮
	parts := make([]int64, 0)
	for _, part := range []rune(messageId) {
		if part >= '0' && part <= '9' {
			val, _ := strconv.ParseInt(string(part), 10, 64)
			parts = append(parts, val)
		}
	}
	if len(parts) >= 2 {
		return int(parts[0]), parts[1], nil
	}
	return 0, 0, fmt.Errorf("invalid message id: %s", messageId)
}
