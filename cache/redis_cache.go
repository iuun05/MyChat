package cache

import (
	"MyChat/global"
	"MyChat/models"
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// 缓存key前缀
	UserCachePrefix      = "user:"
	UserNameCachePrefix  = "userName:"
	FriendsCachePrefix   = "friends:"
	CommunityCachePrefix = "community:"
	OnlineUserPrefix     = "online:"

	// 缓存过期时间
	DefaultExpiration = 30 * time.Minute
	UserExpiration    = 60 * time.Minute
	FriendsExpiration = 15 * time.Minute
	OnlineExpiration  = 5 * time.Minute
)

type RedisCache struct {
	client *redis.Client
	// redis ops need context to contral timeout, cancel, trace req
	ctx context.Context
}

func NewRedisCache() *RedisCache {
	return &RedisCache{
		client: global.RedisDB,
		ctx:    context.Background(),
	}
}

func (r *RedisCache) SetUser(user *models.UserBasic) error {
	key := UserCachePrefix + string(rune(user.ID))
	data, err := json.Marshal(user)
	if err != nil {
		zap.S().Error("序列化用户数据失败：", err)
		return err
	}
	return r.client.Set(r.ctx, key, data, UserExpiration).Err()
}

func (r *RedisCache) GetUser(userID uint) (*models.UserBasic, error) {
	key := UserCachePrefix + string(rune(userID))
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		// cache miss
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var user models.UserBasic
	err = json.Unmarshal([]byte(data), &user)
	if err != nil {
		zap.S().Error("反序列化用户数据失败：", err)
		return nil, err
	}

	return &user, nil
}

func (r *RedisCache) GetUserByName(name string) (*models.UserBasic, error) {
	key := UserNameCachePrefix + name
	// query userID from redis
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		// cache miss
		if err == redis.Nil {
			return nil, nil
		}
		// error
		return nil, err
	}

	var userID64 uint64
	userID64, err = strconv.ParseUint(data, 10, 64)
	if err != nil {
		zap.S().Error("反序列化用户数据失败：", err)
		return nil, err
	}

	userID := uint(userID64)

	// name -> userID
	var user *models.UserBasic
	user, err = r.GetUser(userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *RedisCache) DeleteUser(userID uint) error {
	key := UserCachePrefix + string(rune(userID))
	return r.client.Del(r.ctx, key).Err()
}

func (r *RedisCache) SetFriendsList(userID uint, friends []models.UserBasic) error {
	key := FriendsCachePrefix + string(rune(userID))
	data, err := json.Marshal(friends)
	if err != nil {
		zap.S().Error("序列化用户朋友数据失败：", err)
		return err
	}
	return r.client.Set(r.ctx, key, data, FriendsExpiration).Err()
}

func (r *RedisCache) GetFriendsList(userID uint) ([]models.UserBasic, error) {
	key := FriendsCachePrefix + string(rune(userID))
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// cache miss
			return nil, nil
		}
		return nil, err
	}

	var friends []models.UserBasic
	err = json.Unmarshal([]byte(data), &friends)
	return friends, err
}

func (r *RedisCache) DeleteFriendsList(userID uint) error {
	key := FriendsCachePrefix + string(rune(userID))
	return r.client.Del(r.ctx, key).Err()
}

// 在线用户缓存
func (r *RedisCache) SetUserOnline(userID uint, nodeInfo string) error {
	key := OnlineUserPrefix + string(rune(userID))
	return r.client.Set(r.ctx, key, nodeInfo, OnlineExpiration).Err()
}

func (r *RedisCache) IsUserOnline(userID uint) (bool, error) {
	key := OnlineUserPrefix + string(rune(userID))
	exists, err := r.client.Exists(r.ctx, key).Result()
	return exists > 0, err
}

func (r *RedisCache) SetUserOffline(userID uint) error {
	key := OnlineUserPrefix + string(rune(userID))
	return r.client.Del(r.ctx, key).Err()
}

// 群组缓存
func (r *RedisCache) SetCommunityMembers(communityID uint, memberIDs []uint) error {
	key := CommunityCachePrefix + string(rune(communityID)) + ":members"
	data, err := json.Marshal(memberIDs)
	if err != nil {
		return err
	}

	return r.client.Set(r.ctx, key, data, DefaultExpiration).Err()
}

func (r *RedisCache) GetCommunityMembers(communityID uint) ([]uint, error) {
	key := CommunityCachePrefix + string(rune(communityID)) + ":members"
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var memberIDs []uint
	err = json.Unmarshal([]byte(data), &memberIDs)
	return memberIDs, err
}

// 通用缓存方法
func (r *RedisCache) Set(key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(r.ctx, key, data, expiration).Err()
}

func (r *RedisCache) Get(key string, dest any) error {
	data, err := r.client.Get(r.ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

func (r *RedisCache) Delete(key string) error {
	return r.client.Del(r.ctx, key).Err()
}
