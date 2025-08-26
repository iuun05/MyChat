package dao

import (
	"MyChat/global"
	"MyChat/models"
	"errors"

	"go.uber.org/zap"
)

func FriendList(userId uint) (*[]models.UserBasic, error) {
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

	return &users, nil
}

// add friend by userid
func AddFriend(userId, TargetId uint) (int, error) {
	if userId == TargetId {
		return -2, errors.New("userId == TargetId")
	}

	// add friend by userId
	targetUser, err := FindUserByUserID(TargetId)
	if err != nil {
		return -1, errors.New("no user found")
	}

	if targetUser.ID == 0 {
		zap.S().Info("No user found")
		return -1, errors.New("no user found")
	}

	relation := models.Relation{}

	if tx := global.DB.Where("owner_id = ? and target_id = ? and type = 1", userId, TargetId).First(&relation); tx.RowsAffected == 1 {
		zap.S().Info("The friend exists")
		return 0, errors.New("the friend exists")
	}

	// start transaction
	tx := global.DB.Begin()

	relation.OwnerId = userId
	relation.TargetId = targetUser.ID
	relation.Type = 1

	if t := tx.Create(&relation); t.RowsAffected == 0 {
		zap.S().Info("创建失败")

		//事务回滚
		tx.Rollback()
		return -1, errors.New("创建好友记录失败")
	}

	relation = models.Relation{}
	relation.OwnerId = targetUser.ID
	relation.TargetId = userId
	relation.Type = 1

	if t := tx.Create(&relation); t.RowsAffected == 0 {
		zap.S().Info("创建失败")

		//事务回滚
		tx.Rollback()
		return -1, errors.New("创建好友记录失败")
	}

	tx.Commit()
	return 1, nil
}

func AddFriendByName(userId uint, targetName string) (int, error) {
	user, err := FindUserByName(targetName)
	if err != nil {
		return -1, errors.New("this user does not exist")
	}

	if user.ID == 0 {
		zap.S().Info("the user does not exist")
		return -1, errors.New("the user does not exist")
	}

	return AddFriend(userId, user.ID)
}
