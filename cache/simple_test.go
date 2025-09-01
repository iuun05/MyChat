package cache_test

import (
	"MyChat/cache"
	"MyChat/models"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// 简化测试：只测试核心功能
func TestRedisCache_Simple(t *testing.T) {
	// 创建测试Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   2, // 使用DB 2避免冲突
	})
	defer client.Close()

	// 测试连接
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis未运行，跳过测试")
		return
	}

	// 清空测试数据库
	client.FlushDB(ctx)

	// 创建缓存实例
	rc := cache.NewTestRedisCache(client)

	// 测试1: 基础设置和获取
	t.Run("基础功能", func(t *testing.T) {
		err := rc.SetTest(123)
		assert.NoError(t, err)

		result, err := rc.GetTest(123)
		assert.NoError(t, err)
		assert.Equal(t, "userid", result)
	})

	// 测试2: 用户缓存
	t.Run("用户缓存", func(t *testing.T) {
		user := &models.UserBasic{
			Model: models.Model{
				ID:        1001,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Name:     "测试用户",
			PassWord: "123456",
			Email:    "test@example.com",
		}

		// 设置用户缓存
		err := rc.SetUser(user)
		assert.NoError(t, err)

		// 获取用户缓存
		retrievedUser, err := rc.GetUser(1001)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUser)
		assert.Equal(t, "测试用户", retrievedUser.Name)
		assert.Equal(t, "test@example.com", retrievedUser.Email)

		// 测试通过用户名获取用户（需要先设置用户名映射）
		userIDStr := strconv.FormatUint(uint64(user.ID), 10)
		err = client.Set(ctx, "userName:"+user.Name, userIDStr, 60*time.Minute).Err()
		assert.NoError(t, err)

		retrievedUserByName, err := rc.GetUserByName("测试用户")
		assert.NoError(t, err)
		assert.NotNil(t, retrievedUserByName)
		assert.Equal(t, user.ID, retrievedUserByName.ID)
		assert.Equal(t, user.Name, retrievedUserByName.Name)
	})

	// 测试3: 在线状态
	t.Run("在线状态", func(t *testing.T) {
		userID := uint(2001)

		// 设置在线
		err := rc.SetUserOnline(userID, "node1")
		assert.NoError(t, err)

		// 检查在线状态
		isOnline, err := rc.IsUserOnline(userID)
		assert.NoError(t, err)
		assert.True(t, isOnline)

		// 设置离线
		err = rc.SetUserOffline(userID)
		assert.NoError(t, err)

		// 检查离线状态
		isOnline, err = rc.IsUserOnline(userID)
		assert.NoError(t, err)
		assert.False(t, isOnline)
	})

	// 测试4: 通用缓存
	t.Run("通用缓存", func(t *testing.T) {
		key := "test:key"
		value := "test_value"

		// 设置缓存
		err := rc.Set(key, value, 1*time.Minute)
		assert.NoError(t, err)

		// 获取缓存
		var retrievedValue string
		err = rc.Get(key, &retrievedValue)
		assert.NoError(t, err)
		assert.Equal(t, value, retrievedValue)

		// 删除缓存
		err = rc.Delete(key)
		assert.NoError(t, err)
	})

	// 测试5: 好友列表缓存
	t.Run("好友列表缓存", func(t *testing.T) {
		userID := uint(3001)
		friends := []models.UserBasic{
			{
				Model: models.Model{ID: 3002},
				Name:  "好友1",
			},
			{
				Model: models.Model{ID: 3003},
				Name:  "好友2",
			},
		}

		// 设置好友列表
		err := rc.SetFriendsList(userID, friends)
		assert.NoError(t, err)

		// 获取好友列表
		retrievedFriends, err := rc.GetFriendsList(userID)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedFriends)
		assert.Len(t, retrievedFriends, 2)
		assert.Equal(t, "好友1", retrievedFriends[0].Name)
		assert.Equal(t, "好友2", retrievedFriends[1].Name)
	})

	// 清理测试数据
	client.FlushDB(ctx)
}
