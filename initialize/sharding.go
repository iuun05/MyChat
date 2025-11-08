package initialize

import (
	"MyChat/global"
	"MyChat/models"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// InitShardingTables 初始化分表
// 创建所有分表结构
func InitShardingTables() {
	tableCount := 16 // 分表数量，可以从配置读取

	// 创建私聊消息分表
	for i := 0; i < tableCount; i++ {
		tableName := fmt.Sprintf("message_private_%d", i)
		if err := global.DB.Table(tableName).AutoMigrate(&models.MessagePrivate{}); err != nil {
			zap.S().Errorf("创建私聊消息分表失败 table=%s", tableName, zap.Error(err))
		} else {
			zap.S().Infof("私聊消息分表创建成功: %s", tableName)
		}
	}

	// 创建群聊消息分表
	for i := 0; i < tableCount; i++ {
		tableName := fmt.Sprintf("message_group_%d", i)
		if err := global.DB.Table(tableName).AutoMigrate(&models.MessageGroup{}); err != nil {
			zap.S().Errorf("创建群聊消息分表失败 table=%s", tableName, zap.Error(err))
		} else {
			zap.S().Infof("群聊消息分表创建成功: %s", tableName)
		}
	}

	// 创建群消息已读表
	if err := global.DB.AutoMigrate(&models.MessageGroupRead{}); err != nil {
		zap.S().Error("创建群消息已读表失败", zap.Error(err))
	} else {
		zap.S().Info("群消息已读表创建成功")
	}

	// 创建群成员表
	if err := global.DB.AutoMigrate(&models.GroupMember{}); err != nil {
		zap.S().Error("创建群成员表失败", zap.Error(err))
	} else {
		zap.S().Info("群成员表创建成功")
	}

	zap.S().Info("所有分表初始化完成")
}

// CreateShardingIndexes 创建分表索引
func CreateShardingIndexes() {
	tableCount := 16

	// 为私聊消息表创建索引
	for i := 0; i < tableCount; i++ {
		tableName := fmt.Sprintf("message_private_%d", i)
		createPrivateIndexes(global.DB, tableName)
	}

	// 为群聊消息表创建索引
	for i := 0; i < tableCount; i++ {
		tableName := fmt.Sprintf("message_group_%d", i)
		createGroupIndexes(global.DB, tableName)
	}
}

// createPrivateIndexes 创建私聊消息表索引
func createPrivateIndexes(db *gorm.DB, tableName string) {
	// 复合索引：from_id + target_id
	db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_from_target ON %s(from_id, target_id)
	`, tableName))

	// 索引：seq
	db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_seq ON %s(seq)
	`, tableName))

	// 唯一索引：message_id
	db.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_id ON %s(message_id)
	`, tableName))
}

// createGroupIndexes 创建群聊消息表索引
func createGroupIndexes(db *gorm.DB, tableName string) {
	// 复合索引：group_id + seq
	db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_group_seq ON %s(group_id, seq)
	`, tableName))

	// 索引：from_id
	db.Exec(fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_from ON %s(from_id)
	`, tableName))

	// 唯一索引：message_id
	db.Exec(fmt.Sprintf(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_id ON %s(message_id)
	`, tableName))
}
