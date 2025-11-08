package models

// MessagePrivate 私聊消息表
// 分表策略：按用户ID分表，表名：message_private_{shard}
type MessagePrivate struct {
	Model
	MessageId string `gorm:"type:varchar(64);uniqueIndex:idx_msg_id;not null;comment:消息ID(全局唯一)"` // 全局唯一消息ID
	FromId    int64  `gorm:"index:idx_from_target;not null;comment:发送者ID"`
	TargetId  int64  `gorm:"index:idx_from_target;not null;comment:接收者ID"`
	Seq       int64  `gorm:"index:idx_seq;not null;comment:消息序号(会话内唯一)"`
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
func (MessagePrivate) TableName() string {
	return "message_private" // 基础表名，实际表名由分表工具决定
}
