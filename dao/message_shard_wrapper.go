package dao

import (
	"MyChat/models"
)

// MessageShardDAOWrapper 包装MessageShardDAO以实现kafka接口
type MessageShardDAOWrapper struct {
	*MessageShardDAO
}

// GetGroupMembers 实现kafka.ShardDAO接口
func (w *MessageShardDAOWrapper) GetGroupMembers(groupId int64) ([]interface{}, error) {
	members, err := w.MessageShardDAO.GetGroupMembers(groupId)
	if err != nil {
		return nil, err
	}

	// 转换为接口类型
	result := make([]interface{}, len(members))
	for i, m := range members {
		result[i] = &GroupMemberWrapper{GroupMember: m}
	}
	return result, nil
}

// GroupMemberWrapper 包装models.GroupMember以实现kafka.GroupMember接口
type GroupMemberWrapper struct {
	*models.GroupMember
}

// GetUserId 实现kafka.GroupMember接口
func (w *GroupMemberWrapper) GetUserId() int64 {
	return w.GroupMember.UserId
}

// GetStatus 实现kafka.GroupMember接口
func (w *GroupMemberWrapper) GetStatus() int {
	return w.GroupMember.Status
}
