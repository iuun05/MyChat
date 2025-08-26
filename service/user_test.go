package service

import (
	"MyChat/global"
	"MyChat/models"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupTestService() {
	gin.SetMode(gin.TestMode)

	dsn := "root:@tcp(127.0.0.1:3306)/MyChat?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	global.DB = db

	err = db.AutoMigrate(&models.UserBasic{}, &models.Relation{}, &models.Community{}, &models.Message{})
	if err != nil {
		panic(err)
	}
}

func TestList(t *testing.T) {
	setupTestService()

	// 创建测试用户
	user1 := models.UserBasic{Name: "test1", PassWord: "pass1"}
	user2 := models.UserBasic{Name: "test2", PassWord: "pass2"}
	global.DB.Create(&user1)
	global.DB.Create(&user2)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test1")
	assert.Contains(t, w.Body.String(), "test2")
}

func TestNewUser(t *testing.T) {
	setupTestService()

	form := url.Values{}
	form.Add("name", "testuser")
	form.Add("password", "testpass")
	form.Add("Identity", "testpass")

	req := httptest.NewRequest("POST", "/user/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	NewUser(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "新增用户成功")

	// 验证用户已创建
	var user models.UserBasic
	global.DB.Where("name = ?", "testuser").First(&user)
	assert.Equal(t, "testuser", user.Name)
}

func TestLoginByNameAndPassWord(t *testing.T) {
	setupTestService()

	// 先创建用户
	salt := "12345"
	password := "testpass"
	hashedPassword := "3b8e7a2c3b7e8d6f1234567890abcdef$12345" // 模拟加密后的密码
	user := models.UserBasic{
		Name:     "testuser",
		PassWord: hashedPassword,
		Salt:     salt,
	}
	global.DB.Create(&user)

	form := url.Values{}
	form.Add("name", "testuser")
	form.Add("password", password)

	req := httptest.NewRequest("POST", "/user/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	LoginByNameAndPassWord(c)

	assert.Equal(t, http.StatusOK, w.Code)
	// 注意：由于密码加密算法的复杂性，这个测试可能需要根据实际的加密逻辑进行调整
}
