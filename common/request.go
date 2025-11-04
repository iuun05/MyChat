package common

// BaseRequest 基础请求结构
type BaseRequest struct {
	UserId int64 `json:"userId" binding:"required"` // 用户ID（大多数接口需要）
}

// LoginRequest 登录请求
type LoginRequest struct {
	Name     string `json:"name" binding:"required"`     // 用户名
	Password string `json:"password" binding:"required"` // 密码
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name       string `json:"name" binding:"required"`       // 用户名
	Password   string `json:"password" binding:"required"`   // 密码
	Repassword string `json:"repassword" binding:"required"` // 确认密码（Identity字段）
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	BaseRequest
	Name     string `json:"name"`     // 用户名
	Password string `json:"password"` // 密码
	Email    string `json:"email"`    // 邮箱
	Phone    string `json:"phone"`    // 手机
	Avatar   string `json:"icon"`     // 头像
	Gender   string `json:"gender"`   // 性别
}

// DeleteUserRequest 删除用户请求
type DeleteUserRequest struct {
	BaseRequest
}

// GetUserStatusRequest 获取用户状态请求
type GetUserStatusRequest struct {
	BaseRequest
}

// GetUnreadCountRequest 获取未读消息数请求
type GetUnreadCountRequest struct {
	BaseRequest
}

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	BaseRequest
}

// FriendListRequest 好友列表请求
type FriendListRequest struct {
	BaseRequest
}

// AddFriendRequest 添加好友请求
type AddFriendRequest struct {
	BaseRequest
	TargetId   string `json:"targetId"`   // 目标用户ID或用户名
	TargetName string `json:"targetName"` // 目标用户名（可选）
}

// RemoveFriendRequest 移除好友请求
type RemoveFriendRequest struct {
	BaseRequest
	TargetId int64 `json:"targetId" binding:"required"` // 目标用户ID
}

// GetOnlineFriendsRequest 获取在线好友请求
type GetOnlineFriendsRequest struct {
	BaseRequest
}

// GetRecentMessagesRequest 获取最近消息请求
type GetRecentMessagesRequest struct {
	BaseRequest
	UserIdB int64 `json:"userIdB" binding:"required"` // 对方用户ID
	Limit   int64 `json:"limit"`                      // 限制数量，默认20
}

// GetHistoryRequest 获取历史消息请求
type GetHistoryRequest struct {
	BaseRequest
	UserIdB int64 `json:"userIdB" binding:"required"` // 对方用户ID
	Start   int64 `json:"start"`                      // 起始位置
	End     int64 `json:"end" binding:"required"`     // 结束位置
	IsRev   bool  `json:"isRev"`                      // 是否倒序
}

// NewGroupRequest 新建群请求
type NewGroupRequest struct {
	BaseRequest
	Name string `json:"name" binding:"required"` // 群名称
	Type int    `json:"cate"`                    // 群类型
	Icon string `json:"icon"`                    // 群图标
	Desc string `json:"desc"`                    // 群描述
}

// GroupListRequest 群列表请求
type GroupListRequest struct {
	BaseRequest
}

// JoinGroupRequest 加入群请求
type JoinGroupRequest struct {
	BaseRequest
	ComId string `json:"comId" binding:"required"` // 群ID或群名称
}

// RedisMsgRequest 获取历史消息请求（兼容旧接口）
type RedisMsgRequest struct {
	BaseRequest
	UserIdB int64 `json:"userIdB" binding:"required"` // 对方用户ID
	Start   int64 `json:"start"`                      // 起始位置
	End     int64 `json:"end" binding:"required"`     // 结束位置
	IsRev   bool  `json:"isRev"`                      // 是否倒序
}
