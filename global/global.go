package global

import (
	"MyChat/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	DB            *gorm.DB
	ServiceConfig *config.ServiceConfig
	RedisDB       *redis.Client
)
