package models

import (
	"time"
)

// MessageGroup 群聊消息表
// 分表策略：按群ID分表，表名：message_group_{shard}
// 群聊消息只保存一次，所有成员共享同一条消息记录
type MessageGroup struct {
	Model
	MessageId string `gorm:"type:varchar(64);uniqueIndex:idx_msg_id;not null;comment:消息ID(全局唯一)"` // 全局唯一消息ID
	GroupId   int64  `gorm:"index:idx_group_seq;not null;comment:群ID"`
	FromId    int64  `gorm:"index:idx_from;not null;comment:发送者ID"`
	Seq       int64  `gorm:"index:idx_group_seq;not null;comment:消息序号(群内唯一)"`
	Media     int    `gorm:"default:1;comment:媒体类型:1文字,2图片,3音频,4视频,5文件"`
	Content   string `gorm:"type:text;comment:消息内容"`
	Pic       string `gorm:"type:varchar(512);comment:图片URL"`
	Url       string `gorm:"type:varchar(512);comment:文件URL"`
	Desc      string `gorm:"type:varchar(256);comment:文件描述"`
	Amount    int    `gorm:"default:0;comment:数据大小(字节)"`
	Status    int    `gorm:"default:1;comment:消息状态:1正常,2已撤回,3已删除"`
	// CreatedAt, UpdatedAt, DeletedAt 已通过嵌入 Model 继承，无需重复定义
}

// TableName 动态表名（由分表工具决定）
func (MessageGroup) TableName() string {
	return "message_group" // 基础表名，实际表名由分表工具决定
}

// MessageGroupRead 群消息已读记录表
// 记录每个成员对每条群消息的已读状态
type MessageGroupRead struct {
	Model
	MessageId string    `gorm:"type:varchar(64);index:idx_msg_user;not null;comment:消息ID"`
	GroupId   int64     `gorm:"index:idx_group_user;not null;comment:群ID"`
	UserId    int64     `gorm:"index:idx_msg_user;index:idx_group_user;not null;comment:用户ID"`
	ReadAt    time.Time `gorm:"index;not null;comment:已读时间"`
	// CreatedAt, UpdatedAt 已通过嵌入 Model 继承，无需重复定义
}

// TableName 表名
func (MessageGroupRead) TableName() string {
	return "message_group_read"
}
