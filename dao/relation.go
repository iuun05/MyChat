package dao

import (
	"MyChat/global"
	"MyChat/models"
	"errors"
	"strconv"

	"go.uber.org/zap"
)

// RelationDAO 关系数据访问对象
type RelationDAO struct {
	userDAO *UserDAO // 引用UserDAO用于查找用户
}

// NewRelationDAO 创建RelationDAO实例
func NewRelationDAO() *RelationDAO {
	return &RelationDAO{
		userDAO: NewUserDAO(),
	}
}

// ===== RelationDAO 方法实现 =====

// FriendList 获取好友列表
func (r *RelationDAO) FriendList(userId uint) (*[]models.UserBasic, error) {
	var friends []models.UserBasic
	var err error

	if cache := r.userDAO.getRedisCache(); cache != nil {
		friends, err = cache.GetFriendsList(userId)
		if err != nil {
			zap.S().Warn("[FriendList] Failed to retrieve friend list from cache ", err)
		}

		if len(friends) > 0 {
			zap.S().Info("[FriendList] Friend list cache hits, user ID ", userId)
			return &friends, nil
		}
	}

	// cache miss, query friend relationship from database
	// 查询好友关系
	relation := make([]models.Relation, 0)
	if tx := global.DB.Where("owner_id = ? and type = 1", userId).Find(&relation); tx.RowsAffected == 0 {
		zap.S().Info("未查询到Relation数据")
		return nil, errors.New("未查询到好友关系")
	}

	// 收集好友ID
	userIDs := make([]uint, 0, len(relation))
	for _, v := range relation {
		userIDs = append(userIDs, v.TargetId)
	}

	// 查询好友信息
	users := make([]models.UserBasic, 0)
	if tx := global.DB.Where("id IN ?", userIDs).Find(&users); tx.RowsAffected == 0 {
		zap.S().Info("未查询到好友数据")
		return nil, errors.New("未查到好友")
	}

	// Write result to cache
	if cache := r.userDAO.getRedisCache(); cache != nil {
		if err := cache.SetFriendsList(userId, users); err != nil {
			zap.S().Warn("[FriendList] Failed to write Friend list to cache ", err)
		}
	}

	return &users, nil
}

// AddFriend 添加好友（通过用户ID）
func (r *RelationDAO) AddFriend(userId, TargetId uint) (int, error) {
	if userId == TargetId {
		return -2, errors.New("userId == TargetId")
	}

	// add friend by userId
	targetUser, err := r.userDAO.FindUserByUserID(TargetId)
	if err != nil {
		return -1, errors.New("no user found")
	}

	if targetUser.ID == 0 {
		zap.S().Info("No user found")
		return -1, errors.New("no user found")
	}

	// 先检查好友是否已存在（快速检查，避免不必要的数据库操作）
	relation := models.Relation{}
	if err := global.DB.Where("owner_id = ? and target_id = ? and type = 1", userId, TargetId).First(&relation).Error; err == nil {
		zap.S().Info("The friend exists")
		return 0, errors.New("the friend exists")
	}

	// start transaction
	tx := global.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 在事务中再次检查，防止并发问题
	existingRelation := models.Relation{}
	if err := tx.Where("owner_id = ? and target_id = ? and type = 1", userId, TargetId).First(&existingRelation).Error; err == nil {
		tx.Rollback()
		zap.S().Info("The friend exists (checked in transaction)")
		return 0, errors.New("the friend exists")
	}

	// 创建第一条好友关系（userId -> TargetId）
	relation = models.Relation{
		OwnerId:  userId,
		TargetId: targetUser.ID,
		Type:     1,
	}

	if err := tx.Create(&relation).Error; err != nil {
		zap.S().Error("创建好友记录失败", zap.Error(err))
		tx.Rollback()
		return -1, errors.New("创建好友记录失败: " + err.Error())
	}

	// 创建第二条好友关系（TargetId -> userId），实现双向关系
	reverseRelation := models.Relation{
		OwnerId:  targetUser.ID,
		TargetId: userId,
		Type:     1,
	}

	if err := tx.Create(&reverseRelation).Error; err != nil {
		zap.S().Error("创建反向好友记录失败", zap.Error(err))
		tx.Rollback()
		return -1, errors.New("创建好友记录失败: " + err.Error())
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		zap.S().Error("提交事务失败", zap.Error(err))
		return -1, errors.New("提交事务失败: " + err.Error())
	}

	// 清除缓存
	if cache := r.userDAO.getRedisCache(); cache != nil {
		if err := cache.DeleteFriendsList(userId); err != nil {
			zap.S().Warn("[AddFriend] Failed to clear user friends cache", zap.Error(err))
		}
		if err := cache.DeleteFriendsList(TargetId); err != nil {
			zap.S().Warn("[AddFriend] Failed to clear target user friends cache", zap.Error(err))
		}
	}

	zap.S().Infof("[AddFriend] 成功添加好友 userId=%d, targetId=%d", userId, TargetId)
	return 1, nil
}

// RemoveFriend 移除好友关系
func (r *RelationDAO) RemoveFriend(userId, targetId uint) error {
	tx := global.DB.Begin()

	// 删除双向关系
	if t := tx.Where("owner_id = ? AND target_id = ? AND type = 1", userId, targetId).Delete(&models.Relation{}); t.RowsAffected == 0 {
		tx.Rollback()
		return errors.New("删除好友关系失败")
	}

	if t := tx.Where("owner_id = ? AND target_id = ? AND type = 1", targetId, userId).Delete(&models.Relation{}); t.RowsAffected == 0 {
		tx.Rollback()
		return errors.New("删除好友关系失败")
	}

	tx.Commit()

	// 清除缓存
	if cache := r.userDAO.getRedisCache(); cache != nil {
		if err := cache.DeleteFriendsList(userId); err != nil {
			zap.S().Warn("[RemoveFriend/dao/relation] Failed to clear user friend list cache", err)
		}
		if err := cache.DeleteFriendsList(targetId); err != nil {
			zap.S().Warn("[RemoveFriend/dao/relation] Failed to clear target user friend list cache ", err)
		}
	}

	return nil
}

// AddFriendByName 通过用户名或ID添加好友
func (r *RelationDAO) AddFriendByName(userId uint, targetName string) (int, error) {
	var user *models.UserBasic
	var err error

	// 尝试将targetName解析为数字ID
	if targetID, parseErr := strconv.ParseUint(targetName, 10, 64); parseErr == nil {
		// targetName是数字，使用ID查找
		user, err = r.userDAO.FindUserByUserID(uint(targetID))
		if err != nil {
			zap.S().Infof("[AddFriendByName] 通过ID查找用户失败: %v", err)
			return -1, errors.New("this user does not exist")
		}
	} else {
		// targetName不是数字，使用用户名查找
		user, err = r.userDAO.FindUserByName(targetName)
		if err != nil {
			zap.S().Infof("[AddFriendByName] 通过用户名查找用户失败: %v", err)
			return -1, errors.New("this user does not exist")
		}
	}

	if user == nil || user.ID == 0 {
		zap.S().Info("the user does not exist")
		return -1, errors.New("the user does not exist")
	}

	return r.AddFriend(userId, user.ID)
}

// ===== 向后兼容的全局函数（委托给defaultRelationDAO） =====
var defaultRelationDAO *RelationDAO

func init() {
	defaultRelationDAO = NewRelationDAO()
}

// FriendList 获取好友列表（向后兼容）
func FriendList(userId uint) (*[]models.UserBasic, error) {
	return defaultRelationDAO.FriendList(userId)
}

// AddFriend 添加好友（向后兼容）
func AddFriend(userId, TargetId uint) (int, error) {
	return defaultRelationDAO.AddFriend(userId, TargetId)
}

// RemoveFriend 移除好友关系（向后兼容）
func RemoveFriend(userId, targetId uint) error {
	return defaultRelationDAO.RemoveFriend(userId, targetId)
}

// AddFriendByName 通过用户名或ID添加好友（向后兼容）
func AddFriendByName(userId uint, targetName string) (int, error) {
	return defaultRelationDAO.AddFriendByName(userId, targetName)
}
