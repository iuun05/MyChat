package dao

import (
	"MyChat/cache"
	"MyChat/common"
	"MyChat/global"
	"MyChat/models"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var redisCache = cache.NewRedisCache()

// 获取 user basics 的全部内容
func GetUserList() (userList []*models.UserBasic, err error) {
	if tx := global.DB.Find(&userList); tx.RowsAffected == 0 {
		return nil, errors.New("获取用户列表失败")
	}
	return userList, nil
}

// find user by name and password
func FindUserByNameAndPwd(name string, password string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("name = ? and pass_word = ?", name, password).First(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}

	// 登录识别
	t := strconv.Itoa(int(time.Now().Unix()))
	// MD5
	temp := common.Md5encoder(t)

	if tx := global.DB.Model(&user).Where("id = ?", user.ID).Update("identity", temp); tx.RowsAffected == 0 {
		return nil, errors.New("写入 identity 失败")
	}

	// after login in the system, set user id in cache
	if err := redisCache.SetUser(&user); err != nil {
		zap.S().Warn("[FindUserByNameAndPwd] Failed to update user cache after login ", err)
	}

	// Set user online status
	if err := redisCache.SetUserOnline(user.ID, "logged_in"); err != nil {
		zap.S().Warn("[FindUserByNameAndPwd] Failed to set user online status ", err)
	}

	return &user, nil
}

// find user by name
func FindUserByName(name string) (*models.UserBasic, error) {
	// query from redis
	user, err := redisCache.GetUserByName(name)
	if err != nil {
		zap.S().Warn("[FindUserByName] Failed to retrieve user from cache ", err)
	}

	if user != nil {
		zap.S().Info("[FindUserByName] Cache hit, user ID ", user.ID)
	}

	// query from mysql
	user = &models.UserBasic{}
	if tx := global.DB.Where("name = ?", name).First(user); tx.RowsAffected == 0 {
		return nil, errors.New("查无此人")
	}

	// update redis
	if err := redisCache.SetUser(user); err != nil {
		zap.S().Warn("Fail to update user redis ", err)
	}

	return user, nil
}

// find user, for register
func FindUser(name string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("name = ?", name).First(&user); tx.RowsAffected == 1 {
		return nil, errors.New("用户名已经存在，请换一个用户名")
	}
	return &user, nil
}

// Find user by user id
func FindUserByUserID(ID uint) (*models.UserBasic, error) {
	user, err := redisCache.GetUser(ID)
	// cache miss
	if err != nil {
		zap.S().Warn("redis miss ", err)
	}

	// read from redis
	if user != nil {
		zap.S().Info("redis hit the target ", ID)
		return user, nil
	}

	// query from database
	user = &models.UserBasic{}
	if tx := global.DB.Where(ID).First(user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}

	// Write result to cache
	if err := redisCache.SetUser(user); err != nil {
		zap.S().Warn("fail to write result to cache ", err)
	}

	return user, nil
}

func FindUserByPhone(phone string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("phone = ?", phone).First(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}
	return &user, nil
}

func FindUerByEmail(email string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("email = ?", email).First(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}
	return &user, nil
}

// CreateUser 新建用户
func CreateUser(user models.UserBasic) (*models.UserBasic, error) {
	tx := global.DB.Create(&user)
	if tx.RowsAffected == 0 {
		zap.S().Info("新建用户失败")
		return nil, errors.New("新增用户失败")
	}

	// write new user to redis
	if err := redisCache.SetUser(&user); err != nil {
		zap.S().Warn("[CreateUser] Failed to write new user to cache ", err)
	}

	return &user, nil
}

func UpdateUser(user models.UserBasic) (*models.UserBasic, error) {
	tx := global.DB.Model(&user).Updates(models.UserBasic{
		Name:     user.Name,
		PassWord: user.PassWord,
		Gender:   user.Gender,
		Phone:    user.Phone,
		Email:    user.Email,
		Avatar:   user.Avatar,
		Salt:     user.Salt,
	})
	if tx.RowsAffected == 0 {
		zap.S().Info("更新用户失败")
		return nil, errors.New("更新用户失败")
	}

	// update redis
	if err := redisCache.SetUser(&user); err != nil {
		zap.S().Warn("[UpdateUser] Fail to update cache ", err)
	}

	// delete relative friend list cache
	if err := redisCache.DeleteFriendsList(user.ID); err != nil {
		zap.S().Warn("[UpdateUser] Fail to delete friends' list ", err)
	}

	return &user, nil
}

func DeleteUser(user models.UserBasic) error {
	if tx := global.DB.Delete(&user); tx.RowsAffected == 0 {
		zap.S().Info("删除失败")
		return errors.New("删除用户失败")
	}

	// delete user from cache
	if err := redisCache.DeleteUser(user.ID); err != nil {
		zap.S().Warn("[DeleteUser] Fail to delete user from cache ", err)
	}

	// delete friend list from cache
	if err := redisCache.DeleteFriendsList(user.ID); err != nil {
		zap.S().Warn("[DeleteUser] Fail to delete user friend list from cache ", err)
	}

	return nil
}
