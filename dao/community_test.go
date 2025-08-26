package dao

import (
	"MyChat/global"
	"MyChat/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCommunity(t *testing.T) {
	setupTestDB()

	user := models.UserBasic{Name: "owner", PassWord: "pass"}
	global.DB.Create(&user)

	community := models.Community{
		Name:    "TestGroup",
		OwnerId: user.ID,
		Type:    1,
		Desc:    "Test group description",
	}

	result, err := CreateCommunity(community)
	assert.NoError(t, err)
	assert.Equal(t, 0, result)

	// 验证群组已创建
	var createdCommunity models.Community
	global.DB.Where("name = ?", "TestGroup").First(&createdCommunity)
	assert.Equal(t, "TestGroup", createdCommunity.Name)

	// 测试重复创建群组
	result, err = CreateCommunity(community)
	assert.Error(t, err)
	assert.Equal(t, -1, result)
}

func TestJoinCommunity(t *testing.T) {
	setupTestDB()

	owner := models.UserBasic{Name: "owner", PassWord: "pass"}
	user := models.UserBasic{Name: "user", PassWord: "pass"}
	global.DB.Create(&owner)
	global.DB.Create(&user)

	community := models.Community{
		Name:    "JoinTestGroup",
		OwnerId: owner.ID,
		Type:    1,
	}
	global.DB.Create(&community)

	// 群主加入关系
	relation := models.Relation{
		OwnerId:  owner.ID,
		TargetId: community.ID,
		Type:     2,
	}
	global.DB.Create(&relation)

	// 用户加入群组
	result, err := JoinCommunity(user.ID, "JoinTestGroup")
	assert.NoError(t, err)
	assert.Equal(t, 0, result)

	// 测试重复加入
	result, err = JoinCommunity(user.ID, "JoinTestGroup")
	assert.Error(t, err)
	assert.Equal(t, -1, result)

	// 测试加入不存在的群组
	result, err = JoinCommunity(user.ID, "NonExistentGroup")
	assert.Error(t, err)
	assert.Equal(t, -1, result)
}

func TestGetCommunityList(t *testing.T) {
	setupTestDB()

	user := models.UserBasic{Name: "user", PassWord: "pass"}
	global.DB.Create(&user)

	// 创建群组
	community1 := models.Community{Name: "Group1", OwnerId: user.ID, Type: 1}
	community2 := models.Community{Name: "Group2", OwnerId: user.ID, Type: 1}
	global.DB.Create(&community1)
	global.DB.Create(&community2)

	// 创建关系
	relation1 := models.Relation{OwnerId: user.ID, TargetId: community1.ID, Type: 2}
	relation2 := models.Relation{OwnerId: user.ID, TargetId: community2.ID, Type: 2}
	global.DB.Create(&relation1)
	global.DB.Create(&relation2)

	communities, err := GetCommunityList(user.ID)
	assert.NoError(t, err)
	assert.Len(t, *communities, 2)
}
