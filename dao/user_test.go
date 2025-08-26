package dao

import (
	"MyChat/global"
	"MyChat/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupTestDB() {
	dsn := "root:@tcp(127.0.0.1:3306)/MyChat?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	global.DB = db

	// 自动迁移模式 - 确保测试表结构是最新的
	err = db.AutoMigrate(&models.UserBasic{}, &models.Relation{}, &models.Community{}, &models.Message{})
	if err != nil {
		panic(err)
	}

	// 清理测试数据 - 可选，取决于您是否希望每次测试都清理数据
	// cleanTestData()
}

// cleanTestData 清理测试数据的辅助函数
func cleanTestData() {
	// 删除所有测试相关的数据
	global.DB.Where("name LIKE ?", "test%").Delete(&models.UserBasic{})
	global.DB.Where("name LIKE ?", "Test%").Delete(&models.Community{})
	// 可以根据需要添加更多清理逻辑
}

func TestGetUserList(t *testing.T) {
	setupTestDB()

	// 清理之前的测试数据
	global.DB.Where("name LIKE ?", "testuser%").Delete(&models.UserBasic{})

	// 创建测试用户
	user1 := models.UserBasic{Name: "testuser1", PassWord: "pass1", Email: "test1@example.com"}
	user2 := models.UserBasic{Name: "testuser2", PassWord: "pass2", Email: "test2@example.com"}
	result1 := global.DB.Create(&user1)
	result2 := global.DB.Create(&user2)

	assert.NoError(t, result1.Error)
	assert.NoError(t, result2.Error)

	users, err := GetUserList()
	assert.NoError(t, err)
	assert.NotEmpty(t, users) // 数据库可能有其他用户，所以只检查不为空

	// 验证我们的测试用户存在
	found := false
	for _, user := range users {
		if user.Name == "testuser1" || user.Name == "testuser2" {
			found = true
			break
		}
	}
	assert.True(t, found, "应该找到测试用户")

	// 清理测试数据
	global.DB.Where("name LIKE ?", "testuser%").Delete(&models.UserBasic{})
}

func TestFindUserByName(t *testing.T) {
	setupTestDB()

	// 清理之前的测试数据
	global.DB.Where("name = ?", "testuserbyname").Delete(&models.UserBasic{})

	// 创建测试用户
	user := models.UserBasic{Name: "testuserbyname", PassWord: "testpass", Email: "findtest@example.com"}
	result := global.DB.Create(&user)
	assert.NoError(t, result.Error)

	foundUser, err := FindUserByName("testuserbyname")
	assert.NoError(t, err)
	assert.Equal(t, "testuserbyname", foundUser.Name)

	_, err = FindUserByName("nonexistentuser123456")
	assert.Error(t, err)

	// 清理测试数据
	global.DB.Where("name = ?", "testuserbyname").Delete(&models.UserBasic{})
}

func TestCreateUser(t *testing.T) {
	setupTestDB()

	// 清理之前的测试数据
	global.DB.Where("name = ?", "newusercreate").Delete(&models.UserBasic{})

	user := models.UserBasic{
		Name:     "newusercreate",
		PassWord: "password",
		Email:    "newuser@example.com",
		Phone:    "13800138000",
	}

	createdUser, err := CreateUser(user)
	assert.NoError(t, err)
	assert.Equal(t, "newusercreate", createdUser.Name)
	assert.NotZero(t, createdUser.ID)

	// 清理测试数据
	global.DB.Where("name = ?", "newusercreate").Delete(&models.UserBasic{})
}

func TestUpdateUser(t *testing.T) {
	setupTestDB()

	// 清理和创建测试用户
	global.DB.Where("name = ?", "updatetestuser").Delete(&models.UserBasic{})

	user := models.UserBasic{Name: "updatetestuser", PassWord: "pass", Email: "old@example.com"}
	result := global.DB.Create(&user)
	assert.NoError(t, result.Error)

	// 更新用户信息
	user.Email = "updated@example.com"
	user.Phone = "13800138000"

	updatedUser, err := UpdateUser(user)
	assert.NoError(t, err)
	assert.Equal(t, "updated@example.com", updatedUser.Email)

	// 验证数据库中的数据已更新
	var dbUser models.UserBasic
	global.DB.Where("id = ?", user.ID).First(&dbUser)
	assert.Equal(t, "updated@example.com", dbUser.Email)

	// 清理测试数据
	global.DB.Where("name = ?", "updatetestuser").Delete(&models.UserBasic{})
}

func TestDeleteUser(t *testing.T) {
	setupTestDB()

	// 清理和创建测试用户
	global.DB.Where("name = ?", "deletetestuser").Delete(&models.UserBasic{})

	user := models.UserBasic{Name: "deletetestuser", PassWord: "pass", Email: "delete@example.com"}
	result := global.DB.Create(&user)
	assert.NoError(t, result.Error)

	err := DeleteUser(user)
	assert.NoError(t, err)

	// 验证用户已被软删除（GORM默认是软删除）
	var count int64
	global.DB.Model(&models.UserBasic{}).Where("id = ?", user.ID).Count(&count)
	assert.Equal(t, int64(0), count)
}
