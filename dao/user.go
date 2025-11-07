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
	"gorm.io/gorm"
)

var defaultUserDAO *UserDAO

// UserDAO 用户数据访问对象
type UserDAO struct {
	redisCache *cache.RedisCache
	db         *gorm.DB
}

// NewUserDAO 创建UserDAO实例
func NewUserDAO() *UserDAO {
	if defaultUserDAO == nil {
		defaultUserDAO = &UserDAO{}
		defaultUserDAO.initDependencies()
	}
	return defaultUserDAO
}

// initRedisCache 初始化Redis缓存
func (u *UserDAO) initDependencies() {
	if global.RedisDB != nil && u.redisCache == nil {
		u.redisCache = cache.GetRedisCache()
	}

	if global.DB != nil && u.db == nil {
		u.db = global.DB
	}
}

// getRedisCache 安全地获取Redis缓存实例
func (u *UserDAO) GetRedisCache() *cache.RedisCache {
	if u.redisCache == nil {
		u.initDependencies()
		if u.redisCache == nil {
			zap.S().Warn("Redis缓存未初始化，Redis连接可能有问题")
			return nil
		}
	}
	return u.redisCache
}

// func

// GetUserList 获取 user basics 的全部内容
func (u *UserDAO) GetUserList() (userList []*models.UserBasic, err error) {
	if tx := global.DB.Find(&userList); tx.RowsAffected == 0 {
		return nil, errors.New("获取用户列表失败")
	}
	return userList, nil
}

func (u *UserDAO) FindUserByNameAndPwd(name string, password string) (*models.UserBasic, error) {
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
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.SetUser(&user); err != nil {
			zap.S().Warn("[FindUserByNameAndPwd] Failed to update user cache after login ", err)
		}

		// Set user online status
		if err := cache.SetUserOnline(user.ID, "logged_in"); err != nil {
			zap.S().Warn("[FindUserByNameAndPwd] Failed to set user online status ", err)
		}
	}

	return &user, nil
}

// FindUserByName 根据用户名查找用户
func (u *UserDAO) FindUserByName(name string) (*models.UserBasic, error) {
	var user *models.UserBasic
	var err error

	// query from redis
	if cache := u.GetRedisCache(); cache != nil {
		user, err = cache.GetUserByName(name)
		if err != nil {
			zap.S().Warn("[FindUserByName] Failed to retrieve user from cache ", err)
		}

		if user != nil {
			zap.S().Info("[FindUserByName] Cache hit, user ID ", user.ID)
			return user, nil
		}
	}

	// query from mysql
	user = &models.UserBasic{}
	if tx := global.DB.Where("name = ?", name).First(user); tx.RowsAffected == 0 {
		return nil, errors.New("查无此人")
	}

	// update redis
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.SetUser(user); err != nil {
			zap.S().Warn("Fail to update user redis ", err)
		}
	}

	return user, nil
}

// FindUser 查找用户（用于注册时检查用户名是否存在）
func (u *UserDAO) FindUser(name string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("name = ?", name).First(&user); tx.RowsAffected == 1 {
		return nil, errors.New("用户名已经存在，请换一个用户名")
	}
	return &user, nil
}

// FindUserByUserID 根据用户ID查找用户
func (u *UserDAO) FindUserByUserID(ID uint) (*models.UserBasic, error) {
	var user *models.UserBasic
	var err error

	// query from redis
	if cache := u.GetRedisCache(); cache != nil {
		user, err = cache.GetUser(ID)
		// cache miss
		if err != nil {
			zap.S().Warn("redis miss ", err)
		}

		// read from redis
		if user != nil {
			zap.S().Info("redis hit the target ", ID)
			return user, nil
		}
	}

	// query from database
	user = &models.UserBasic{}
	if tx := global.DB.Where("id = ?", ID).First(user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}

	// Write result to cache
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.SetUser(user); err != nil {
			zap.S().Warn("fail to write result to cache ", err)
		}
	}

	return user, nil
}

// FindUserByPhone 根据手机号查找用户
func (u *UserDAO) FindUserByPhone(phone string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("phone = ?", phone).First(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}
	return &user, nil
}

// FindUserByEmail 根据邮箱查找用户
func (u *UserDAO) FindUserByEmail(email string) (*models.UserBasic, error) {
	user := models.UserBasic{}
	if tx := global.DB.Where("email = ?", email).First(&user); tx.RowsAffected == 0 {
		return nil, errors.New("未查询到记录")
	}
	return &user, nil
}

// CreateUser 新建用户
func (u *UserDAO) CreateUser(user models.UserBasic) (*models.UserBasic, error) {
	tx := global.DB.Create(&user)
	if tx.RowsAffected == 0 {
		zap.S().Info("新建用户失败")
		return nil, errors.New("新增用户失败")
	}

	// write new user to redis
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.SetUser(&user); err != nil {
			zap.S().Warn("[CreateUser] Failed to write new user to cache ", err)
		}
	}

	return &user, nil
}

// UpdateUser 更新用户信息
func (u *UserDAO) UpdateUser(user models.UserBasic) (*models.UserBasic, error) {
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
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.SetUser(&user); err != nil {
			zap.S().Warn("[UpdateUser] Fail to update cache ", err)
		}

		// delete relative friend list cache
		if err := cache.DeleteFriendsList(user.ID); err != nil {
			zap.S().Warn("[UpdateUser] Fail to delete friends' list ", err)
		}
	}

	return &user, nil
}

// DeleteUser 删除用户
func (u *UserDAO) DeleteUser(user models.UserBasic) error {
	if tx := global.DB.Delete(&user); tx.RowsAffected == 0 {
		zap.S().Info("删除失败")
		return errors.New("删除用户失败")
	}

	// delete user from cache
	if cache := u.GetRedisCache(); cache != nil {
		if err := cache.DeleteUser(user.ID); err != nil {
			zap.S().Warn("[DeleteUser] Fail to delete user from cache ", err)
		}

		// delete friend list from cache
		if err := cache.DeleteFriendsList(user.ID); err != nil {
			zap.S().Warn("[DeleteUser] Fail to delete user friend list from cache ", err)
		}
	}

	return nil
}
