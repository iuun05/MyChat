package cache_test

import (
	"MyChat/cache"
	"MyChat/models"
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试用的Redis客户端
var testRedisClient *redis.Client

// 测试用的缓存实例
var testCache *cache.RedisCache

func setupTestRedis() {
	// 初始化测试Redis客户端
	testRedisClient = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   1, // 使用不同的数据库避免影响生产数据
	})

	// 测试连接
	ctx := context.Background()
	err := testRedisClient.Ping(ctx).Err()
	if err != nil {
		panic("无法连接到Redis: " + err.Error())
	}

	// 清空测试数据库
	testRedisClient.FlushDB(ctx)

	// 创建缓存实例
	testCache = cache.NewTestRedisCache(testRedisClient)
}

func teardownTestRedis() {
	if testRedisClient != nil {
		testRedisClient.Close()
	}
}

// 创建测试用户数据
func createTestUser(id uint, name string) *models.UserBasic {
	return &models.UserBasic{
		Model: models.Model{
			ID:        id,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:     name,
		PassWord: "test123",
		Avatar:   "avatar.jpg",
		Gender:   "male",
		Phone:    "13800138000",
		Email:    "test@example.com",
		Identity: "test_identity",
		ClientIp: "127.0.0.1",
		Salt:     "test_salt",
	}
}

// 创建测试好友列表
func createTestFriends() []models.UserBasic {
	return []models.UserBasic{
		*createTestUser(1001, "Friend1"),
		*createTestUser(1002, "Friend2"),
		*createTestUser(1003, "Friend3"),
	}
}

func TestMain(m *testing.M) {
	setupTestRedis()
	defer teardownTestRedis()
	m.Run()
}

// 测试基础设置和获取功能
func TestRedisCache_BasicSetAndGet(t *testing.T) {
	// 测试设置和获取
	err := testCache.SetTest(12345)
	require.NoError(t, err)

	result, err := testCache.GetTest(12345)
	require.NoError(t, err)
	assert.Equal(t, "userid", result)
}

// 测试用户缓存功能
func TestRedisCache_UserOperations(t *testing.T) {
	// 创建测试用户
	user := createTestUser(1001, "TestUser")

	// 测试设置用户缓存
	err := testCache.SetUser(user)
	require.NoError(t, err)

	// 测试获取用户缓存
	retrievedUser, err := testCache.GetUser(1001)
	require.NoError(t, err)
	require.NotNil(t, retrievedUser)
	assert.Equal(t, user.Name, retrievedUser.Name)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.Phone, retrievedUser.Phone)

	// 测试获取不存在的用户
	nonExistentUser, err := testCache.GetUser(9999)
	require.NoError(t, err)
	assert.Nil(t, nonExistentUser)

	// 测试删除用户缓存
	err = testCache.DeleteUser(1001)
	require.NoError(t, err)

	// 验证用户已被删除
	deletedUser, err := testCache.GetUser(1001)
	require.NoError(t, err)
	assert.Nil(t, deletedUser)
}

// 测试通过用户名获取用户
func TestRedisCache_GetUserByName(t *testing.T) {
	// 创建测试用户
	user := createTestUser(1002, "Alice")

	// 设置用户缓存
	err := testCache.SetUser(user)
	require.NoError(t, err)

	// 设置用户名到用户ID的映射
	userIDStr := strconv.FormatUint(uint64(user.ID), 10)
	err = testRedisClient.Set(context.Background(), "userName:"+user.Name, userIDStr, 60*time.Minute).Err()
	require.NoError(t, err)

	// 测试通过用户名获取用户
	retrievedUser, err := testCache.GetUserByName("Alice")
	require.NoError(t, err)
	require.NotNil(t, retrievedUser)
	assert.Equal(t, user.ID, retrievedUser.ID)
	assert.Equal(t, user.Name, retrievedUser.Name)
}

// 测试好友列表缓存功能
func TestRedisCache_FriendsListOperations(t *testing.T) {
	userID := uint(2001)
	friends := createTestFriends()

	// 测试设置好友列表
	err := testCache.SetFriendsList(userID, friends)
	require.NoError(t, err)

	// 测试获取好友列表
	retrievedFriends, err := testCache.GetFriendsList(userID)
	require.NoError(t, err)
	require.NotNil(t, retrievedFriends)
	assert.Len(t, retrievedFriends, 3)
	assert.Equal(t, "Friend1", retrievedFriends[0].Name)
	assert.Equal(t, "Friend2", retrievedFriends[1].Name)
	assert.Equal(t, "Friend3", retrievedFriends[2].Name)

	// 测试删除好友列表
	err = testCache.DeleteFriendsList(userID)
	require.NoError(t, err)

	// 验证好友列表已被删除
	deletedFriends, err := testCache.GetFriendsList(userID)
	require.NoError(t, err)
	assert.Nil(t, deletedFriends)
}

// 测试在线状态缓存功能
func TestRedisCache_OnlineStatusOperations(t *testing.T) {
	userID := uint(3001)
	nodeInfo := "node1.example.com:8080"

	// 测试设置用户在线状态
	err := testCache.SetUserOnline(userID, nodeInfo)
	require.NoError(t, err)

	// 测试检查用户是否在线
	isOnline, err := testCache.IsUserOnline(userID)
	require.NoError(t, err)
	assert.True(t, isOnline)

	// 测试设置用户离线
	err = testCache.SetUserOffline(userID)
	require.NoError(t, err)

	// 验证用户已离线
	isOnline, err = testCache.IsUserOnline(userID)
	require.NoError(t, err)
	assert.False(t, isOnline)
}

// 测试群组缓存功能
func TestRedisCache_CommunityOperations(t *testing.T) {
	communityID := uint(4001)
	memberIDs := []uint{1001, 1002, 1003, 1004}

	// 测试设置群组成员
	err := testCache.SetCommunityMembers(communityID, memberIDs)
	require.NoError(t, err)

	// 测试获取群组成员
	retrievedMembers, err := testCache.GetCommunityMembers(communityID)
	require.NoError(t, err)
	require.NotNil(t, retrievedMembers)
	assert.Len(t, retrievedMembers, 4)
	assert.Contains(t, retrievedMembers, uint(1001))
	assert.Contains(t, retrievedMembers, uint(1002))
	assert.Contains(t, retrievedMembers, uint(1003))
	assert.Contains(t, retrievedMembers, uint(1004))
}

// 测试通用缓存方法
func TestRedisCache_GenericOperations(t *testing.T) {
	key := "test:generic:key"
	value := map[string]interface{}{
		"name":   "TestValue",
		"count":  42,
		"active": true,
	}
	expiration := 5 * time.Minute

	// 测试设置通用缓存
	err := testCache.Set(key, value, expiration)
	require.NoError(t, err)

	// 测试获取通用缓存
	var retrievedValue map[string]interface{}
	err = testCache.Get(key, &retrievedValue)
	require.NoError(t, err)
	assert.Equal(t, value["name"], retrievedValue["name"])
	// JSON序列化会将int转换为float64
	assert.Equal(t, float64(42), retrievedValue["count"])
	assert.Equal(t, value["active"], retrievedValue["active"])

	// 测试删除通用缓存
	err = testCache.Delete(key)
	require.NoError(t, err)

	// 验证缓存已被删除
	var deletedValue map[string]interface{}
	err = testCache.Get(key, &deletedValue)
	assert.Error(t, err)
}

// 测试缓存过期时间
func TestRedisCache_Expiration(t *testing.T) {
	key := "test:expiration:key"
	value := "test_value"
	expiration := 1 * time.Second

	// 设置短期缓存
	err := testCache.Set(key, value, expiration)
	require.NoError(t, err)

	// 立即获取应该成功
	var retrievedValue string
	err = testCache.Get(key, &retrievedValue)
	require.NoError(t, err)
	assert.Equal(t, value, retrievedValue)

	// 等待过期
	time.Sleep(2 * time.Second)

	// 过期后获取应该失败
	err = testCache.Get(key, &retrievedValue)
	assert.Error(t, err)
}

// 测试并发操作
func TestRedisCache_ConcurrentOperations(t *testing.T) {
	const numGoroutines = 10
	const numOperations = 100

	// 并发设置缓存
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("concurrent:test:%d:%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				err := testCache.Set(key, value, 1*time.Minute)
				if err != nil {
					t.Errorf("并发设置缓存失败: %v", err)
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	time.Sleep(2 * time.Second)

	// 验证缓存设置成功
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numOperations; j++ {
			key := fmt.Sprintf("concurrent:test:%d:%d", i, j)
			expectedValue := fmt.Sprintf("value_%d_%d", i, j)

			var retrievedValue string
			err := testCache.Get(key, &retrievedValue)
			if err != nil {
				t.Errorf("获取缓存失败: %v", err)
			}
			if retrievedValue != expectedValue {
				t.Errorf("缓存值不匹配，期望: %s, 实际: %s", expectedValue, retrievedValue)
			}
		}
	}
}

// 测试错误情况
func TestRedisCache_ErrorCases(t *testing.T) {
	// 测试设置nil值
	err := testCache.Set("test:nil", nil, 1*time.Minute)
	require.NoError(t, err)

	// 测试获取到nil值
	var retrievedValue interface{}
	err = testCache.Get("test:nil", &retrievedValue)
	require.NoError(t, err)
	assert.Nil(t, retrievedValue)

	// 测试删除不存在的key
	err = testCache.Delete("non:existent:key")
	require.NoError(t, err) // Redis删除不存在的key不会返回错误
}

// 测试性能基准
func BenchmarkRedisCache_SetUser(b *testing.B) {
	user := createTestUser(9999, "BenchmarkUser")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user.ID = uint(i)
		err := testCache.SetUser(user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRedisCache_GetUser(b *testing.B) {
	// 预先设置一些测试数据
	user := createTestUser(8888, "BenchmarkUser")
	err := testCache.SetUser(user)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := testCache.GetUser(8888)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 测试数据清理
func TestRedisCache_Cleanup(t *testing.T) {
	// 清理所有测试数据
	ctx := context.Background()
	err := testRedisClient.FlushDB(ctx).Err()
	require.NoError(t, err)

	// 验证清理成功
	keys, err := testRedisClient.Keys(ctx, "*").Result()
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}
