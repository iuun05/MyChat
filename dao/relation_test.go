package dao

import (
	"MyChat/global"
	"MyChat/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddFriend(t *testing.T) {
	setupTestDB()

	// 创建两个测试用户
	user1 := models.UserBasic{Name: "user1", PassWord: "pass1"}
	user2 := models.UserBasic{Name: "user2", PassWord: "pass2"}
	global.DB.Create(&user1)
	global.DB.Create(&user2)

	// 添加好友关系
	result, err := AddFriend(user1.ID, user2.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	// 验证好友关系是否创建成功
	var relations []models.Relation
	global.DB.Where("owner_id = ? AND target_id = ? AND type = 1", user1.ID, user2.ID).Find(&relations)
	assert.Len(t, relations, 1)

	// 测试重复添加好友
	result, err = AddFriend(user1.ID, user2.ID)
	assert.Error(t, err)
	assert.Equal(t, 0, result)
}

func TestAddFriendByName(t *testing.T) {
	setupTestDB()

	user1 := models.UserBasic{Name: "user1", PassWord: "pass1"}
	user2 := models.UserBasic{Name: "user2", PassWord: "pass2"}
	global.DB.Create(&user1)
	global.DB.Create(&user2)

	result, err := AddFriendByName(user1.ID, "user2")
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	// 测试添加不存在的用户
	result, err = AddFriendByName(user1.ID, "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, -1, result)
}

func TestFriendList(t *testing.T) {
	setupTestDB()

	user1 := models.UserBasic{Name: "user1", PassWord: "pass1"}
	user2 := models.UserBasic{Name: "user2", PassWord: "pass2"}
	user3 := models.UserBasic{Name: "user3", PassWord: "pass3"}
	global.DB.Create(&user1)
	global.DB.Create(&user2)
	global.DB.Create(&user3)

	// 添加好友关系
	AddFriend(user1.ID, user2.ID)
	AddFriend(user1.ID, user3.ID)

	friends, err := FriendList(user1.ID)
	assert.NoError(t, err)
	assert.Len(t, *friends, 2)
}
