package dao

import (
	"MyChat/global"
	"MyChat/models"
	"MyChat/utils"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// MessageShardDAO 分表消息数据访问对象
type MessageShardDAO struct {
	sharding  *utils.TableSharding
	snowflake *utils.Snowflake
}

// NewMessageShardDAO 创建分表消息DAO
func NewMessageShardDAO() *MessageShardDAO {
	return &MessageShardDAO{
		sharding:  utils.NewTableSharding(16), // 默认16张表
		snowflake: utils.NewSnowflake(1, 1),   // 机器ID和数据中心ID从配置读取
	}
}

// SavePrivateMessage 保存私聊消息
func (m *MessageShardDAO) SavePrivateMessage(fromId, targetId int64, content string, media int, seq int64) (*models.MessagePrivate, error) {
	// 获取分表名
	tableName := m.sharding.GetPrivateTableName(fromId, targetId)

	// 生成消息ID
	messageId := m.snowflake.GenerateMessageID()

	msg := &models.MessagePrivate{
		MessageId: messageId,
		FromId:    fromId,
		TargetId:  targetId,
		Seq:       seq,
		Media:     media,
		Content:   content,
		Status:    1,
		// CreatedAt, UpdatedAt 由 GORM 自动处理
	}

	// 使用Table方法指定表名
	if err := global.DB.Table(tableName).Create(msg).Error; err != nil {
		zap.S().Errorf("[SavePrivateMessage] 保存私聊消息失败 table=%s, fromId=%d, targetId=%d",
			tableName, fromId, targetId, zap.Error(err))
		return nil, err
	}

	zap.S().Debugf("[SavePrivateMessage] 私聊消息已保存 table=%s, messageId=%s, seq=%d",
		tableName, messageId, seq)
	return msg, nil
}

// SaveGroupMessage 保存群聊消息（只保存一次）
func (m *MessageShardDAO) SaveGroupMessage(groupId, fromId int64, content string, media int, seq int64) (*models.MessageGroup, error) {
	// 获取分表名
	tableName := m.sharding.GetGroupTableName(groupId)

	// 生成消息ID
	messageId := m.snowflake.GenerateMessageID()

	msg := &models.MessageGroup{
		MessageId: messageId,
		GroupId:   groupId,
		FromId:    fromId,
		Seq:       seq,
		Media:     media,
		Content:   content,
		Status:    1,
		// CreatedAt, UpdatedAt 由 GORM 自动处理
	}

	// 使用Table方法指定表名
	if err := global.DB.Table(tableName).Create(msg).Error; err != nil {
		zap.S().Errorf("[SaveGroupMessage] 保存群聊消息失败 table=%s, groupId=%d",
			tableName, groupId, zap.Error(err))
		return nil, err
	}

	zap.S().Debugf("[SaveGroupMessage] 群聊消息已保存 table=%s, messageId=%s, seq=%d",
		tableName, messageId, seq)
	return msg, nil
}

// GetPrivateMessages 获取私聊消息列表
func (m *MessageShardDAO) GetPrivateMessages(userId1, userId2 int64, limit, offset int) ([]*models.MessagePrivate, error) {
	tableName := m.sharding.GetPrivateTableName(userId1, userId2)

	var messages []*models.MessagePrivate
	query := global.DB.Table(tableName).
		Where("((from_id = ? AND target_id = ?) OR (from_id = ? AND target_id = ?)) AND status = 1",
			userId1, userId2, userId2, userId1).
		Order("seq DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&messages).Error; err != nil {
		zap.S().Errorf("[GetPrivateMessages] 查询私聊消息失败 table=%s", tableName, zap.Error(err))
		return nil, err
	}

	return messages, nil
}

// GetGroupMessages 获取群聊消息列表
func (m *MessageShardDAO) GetGroupMessages(groupId int64, limit, offset int) ([]*models.MessageGroup, error) {
	tableName := m.sharding.GetGroupTableName(groupId)

	var messages []*models.MessageGroup
	query := global.DB.Table(tableName).
		Where("group_id = ? AND status = 1", groupId).
		Order("seq DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&messages).Error; err != nil {
		zap.S().Errorf("[GetGroupMessages] 查询群聊消息失败 table=%s", tableName, zap.Error(err))
		return nil, err
	}

	return messages, nil
}

// MarkGroupMessageRead 标记群消息已读
func (m *MessageShardDAO) MarkGroupMessageRead(messageId string, groupId, userId int64) error {
	readRecord := &models.MessageGroupRead{
		MessageId: messageId,
		GroupId:   groupId,
		UserId:    userId,
		ReadAt:    time.Now(),
		// CreatedAt, UpdatedAt 由 GORM 自动处理
	}

	// 使用ON DUPLICATE KEY UPDATE避免重复
	if err := global.DB.Where("message_id = ? AND user_id = ?", messageId, userId).
		FirstOrCreate(readRecord).Error; err != nil {
		zap.S().Errorf("[MarkGroupMessageRead] 标记已读失败 messageId=%s, userId=%d",
			messageId, userId, zap.Error(err))
		return err
	}

	return nil
}

// GetUnreadGroupMessageCount 获取群未读消息数
func (m *MessageShardDAO) GetUnreadGroupMessageCount(groupId, userId int64, lastReadSeq int64) (int64, error) {
	tableName := m.sharding.GetGroupTableName(groupId)

	var count int64
	query := global.DB.Table(tableName).
		Where("group_id = ? AND seq > ? AND status = 1", groupId, lastReadSeq).
		Count(&count)

	if err := query.Error; err != nil {
		zap.S().Errorf("[GetUnreadGroupMessageCount] 查询未读数失败 groupId=%d", groupId, zap.Error(err))
		return 0, err
	}

	return count, nil
}

// GetGroupMaxSeq 获取群最大序号
func (m *MessageShardDAO) GetGroupMaxSeq(groupId int64) (int64, error) {
	tableName := m.sharding.GetGroupTableName(groupId)

	var maxSeq int64
	if err := global.DB.Table(tableName).
		Where("group_id = ?", groupId).
		Select("COALESCE(MAX(seq), 0)").
		Scan(&maxSeq).Error; err != nil {
		zap.S().Errorf("[GetGroupMaxSeq] 查询最大序号失败 groupId=%d", groupId, zap.Error(err))
		return 0, err
	}

	return maxSeq, nil
}

// SaveGroupMessageWithCache 保存群消息并更新缓存
func (m *MessageShardDAO) SaveGroupMessageWithCache(groupId, fromId int64, content string, media int, seq int64) (*models.MessageGroup, error) {
	// 保存到数据库
	msg, err := m.SaveGroupMessage(groupId, fromId, content, media, seq)
	if err != nil {
		return nil, err
	}

	// 更新Redis缓存
	ctx := context.Background()
	key := fmt.Sprintf("group_msg:%d", groupId)
	msgData := strconv.FormatUint(uint64(msg.ID), 10)

	// 使用ZSet存储，score为seq，member为消息ID
	if err := global.RedisDB.ZAdd(ctx, key, redis.Z{
		Score:  float64(seq),
		Member: msgData,
	}).Err(); err != nil {
		zap.S().Warnf("[SaveGroupMessageWithCache] 更新Redis缓存失败 groupId=%d", groupId)
	}

	// 限制缓存大小（只保留最近1000条）
	if count, _ := global.RedisDB.ZCard(ctx, key).Result(); count > 1000 {
		global.RedisDB.ZRemRangeByRank(ctx, key, 0, count-1000)
	}

	return msg, nil
}

// GetGroupMembers 获取群成员列表
func (m *MessageShardDAO) GetGroupMembers(groupId int64) ([]*models.GroupMember, error) {
	var members []*models.GroupMember
	if err := global.DB.Where("group_id = ? AND status = ?",
		groupId, models.GroupMemberStatusNormal).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// 全局实例
var defaultMessageShardDAO *MessageShardDAO

func init() {
	defaultMessageShardDAO = NewMessageShardDAO()
}
