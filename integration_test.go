package main

import (
	"MyChat/dao"
	"MyChat/global"
	"MyChat/models"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupIntegrationTest() {
	dsn := "root:@tcp(127.0.0.1:3306)/MyChat?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	global.DB = db

	// 自动迁移表
	err = db.AutoMigrate(&models.UserBasic{}, &models.Relation{}, &models.Community{}, &models.Message{})
	if err != nil {
		panic(err)
	}

	// 清空表，保证测试干净
	global.DB.Exec("DELETE FROM relations")
	global.DB.Exec("DELETE FROM user_basics")
	global.DB.Exec("DELETE FROM communities")
	global.DB.Exec("DELETE FROM messages")
}

// 修复 FriendList 传指针问题
func FriendListFixed(userId uint) (*[]models.UserBasic, error) {
	relation := make([]models.Relation, 0)
	if tx := global.DB.Where("owner_id = ? AND type = 1", userId).Find(&relation); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到好友关系")
	}

	UserID := make([]uint, 0)
	for _, v := range relation {
		UserID = append(UserID, v.TargetId)
	}

	user := make([]models.UserBasic, 0)
	if tx := global.DB.Where("id IN ?", UserID).Find(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查到好友")
	}
	return &user, nil
}

func TestUserFriendWorkflow(t *testing.T) {
	setupIntegrationTest()

	// 1. 创建两个用户
	user1 := models.UserBasic{Name: "alice", PassWord: "pass1", Email: "alice@example.com"}
	user2 := models.UserBasic{Name: "bob", PassWord: "pass2", Email: "bob@example.com"}

	createdUser1, err := dao.CreateUser(user1)
	assert.NoError(t, err)
	createdUser2, err := dao.CreateUser(user2)
	assert.NoError(t, err)

	// 2. 添加好友关系
	result, err := dao.AddFriend(createdUser1.ID, createdUser2.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	// 3. 获取好友列表
	friends, err := FriendListFixed(createdUser1.ID)
	assert.NoError(t, err)
	assert.Len(t, *friends, 1)
	assert.Equal(t, "bob", (*friends)[0].Name)

	// 4. 创建群组
	community := models.Community{
		Name:    "TestGroup",
		OwnerId: createdUser1.ID,
		Type:    1,
		Desc:    "Integration test group",
	}

	result, err = dao.CreateCommunity(community)
	assert.NoError(t, err)
	assert.Equal(t, 0, result)

	// 5. 用户2加入群组
	result, err = dao.JoinCommunity(createdUser2.ID, "TestGroup")
	assert.NoError(t, err)
	assert.Equal(t, 0, result)

	// 6. 获取群组列表
	communities, err := dao.GetCommunityList(createdUser1.ID)
	assert.NoError(t, err)
	assert.Len(t, *communities, 1)
	assert.Equal(t, "TestGroup", (*communities)[0].Name)
}
