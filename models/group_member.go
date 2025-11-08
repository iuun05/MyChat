package models

import (
	"time"
)

// GroupMember 群成员表
// 优化后的群成员关系表，替代Relation表中type=2的记录
type GroupMember struct {
	Model
	GroupId   int64      `gorm:"index:idx_group_user;not null;comment:群ID"`
	UserId    int64      `gorm:"index:idx_group_user;not null;comment:用户ID"`
	Role      int        `gorm:"default:0;comment:角色:0普通成员,1管理员,2群主"`
	Nickname  string     `gorm:"type:varchar(64);comment:群昵称"`
	JoinAt    time.Time  `gorm:"not null;comment:加入时间"`
	MuteUntil *time.Time `gorm:"comment:禁言到期时间"`
	Status    int        `gorm:"default:1;comment:状态:1正常,2已退出,3被踢出"`
	// CreatedAt, UpdatedAt, DeletedAt 已通过嵌入 Model 继承，无需重复定义
}

// TableName 表名
func (GroupMember) TableName() string {
	return "group_member"
}

// 群成员角色常量
const (
	GroupRoleMember = 0 // 普通成员
	GroupRoleAdmin  = 1 // 管理员
	GroupRoleOwner  = 2 // 群主
)

// 群成员状态常量
const (
	GroupMemberStatusNormal = 1 // 正常
	GroupMemberStatusQuit   = 2 // 已退出
	GroupMemberStatusKicked = 3 // 被踢出
)
