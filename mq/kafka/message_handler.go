package kafka

// MessageHandler 消息处理器接口（避免循环依赖）
type MessageHandler interface {
	// PushMessageToUser 推送消息给用户
	PushMessageToUser(userId int64, msg []byte)
	// GetShardDAO 获取分表DAO
	GetShardDAO() ShardDAO
}

// ShardDAO 分表DAO接口
type ShardDAO interface {
	GetGroupMembers(groupId int64) ([]interface{}, error)
}

// GroupMember 群成员接口（避免循环依赖）
type GroupMember interface {
	GetUserId() uint
	GetStatus() int
}
