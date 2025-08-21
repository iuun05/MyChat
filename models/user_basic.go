package models

import (
	"time"

	"gorm.io/gorm"
)

// gorm.Model 的定义
type Model struct {
	// 主键
	ID uint `gorm:"primaryKey"`
	// 在创建记录时自动设置为当前时间。
	CreatedAt time.Time
	// 每当记录更新时，自动更新为当前时间。
	UpdatedAt time.Time
	// 用于软删除（将记录标记为已删除，而实际上并未从数据库中删除）。
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type UserBasic struct {
	Model
	Name     string
	PassWord string
	// 头像
	Avatar        string
	Gender        string `gorm:"column:gender;default:male;type:varchar(6) comment 'male表示男, famale表示女'"` //gorm为数据库字段约束
	Phone         string `valid:"matches(^1[3-9]{1}\\d{9}$)"`                                             //valid为条件约束
	Email         string `valid:"email"`
	Identity      string
	ClientIp      string `valid:"ipv4"` // vaild 用于验证是否符合相关格式
	ClientPort    string
	Salt          string     //盐值
	LoginTime     *time.Time `gorm:"column:login_time"`
	HeartBeatTime *time.Time `gorm:"column:heart_beat_time"`
	LoginOutTime  *time.Time `gorm:"column:login_out_time"`
	IsLoginOut    bool
	DeviceInfo    string //登录设备
}

// 指定表的名称
func (table *UserBasic) UserTableName() string {
	return "user_basic"
}

type Relation struct {
	Model
	OwnerId  uint   // 谁的关系信息
	TargetId uint   // 对应的谁
	Type     int    // 关系描述： 1. 好友关系；2. 群关系
	Desc     string // 描述
}

func (r *Relation) RelTableName() string {
	return "relation"
}
