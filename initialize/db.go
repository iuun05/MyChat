package initialize

import (
	"MyChat/cache"
	"MyChat/global"
	"MyChat/models"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", global.ServiceConfig.DB.User,
		global.ServiceConfig.DB.Password, global.ServiceConfig.DB.Host, global.ServiceConfig.DB.Port, global.ServiceConfig.DB.Name)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Silent, // Log level
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,          // Don't include params in the SQL log
			Colorful:                  false,         // Disable color
		},
	)

	var err error
	global.DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		zap.S().Error("mysql init failed", zap.Error(err))
		panic(err)
	}

	// 配置数据库连接池，避免在高并发下频繁建连导致延迟抖动
	sqlDB, err := global.DB.DB()
	if err != nil {
		zap.S().Error("get sql.DB from gorm failed", zap.Error(err))
		panic(err)
	}
	// 根据你本机 MySQL 能力适当调节
	sqlDB.SetMaxOpenConns(100)                 // 最大连接数
	sqlDB.SetMaxIdleConns(50)                  // 最大空闲连接数
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // 连接最长存活时间

	// 自动迁移数据库表结构，确保表结构与模型定义一致
	err = global.DB.AutoMigrate(
		&models.UserBasic{},
		&models.Relation{},
		&models.Community{},
		&models.Message{}, // 保留旧表，用于兼容
	)
	if err != nil {
		zap.S().Error("数据库迁移失败", zap.Error(err))
		panic(err)
	}

	// 初始化分表
	InitShardingTables()

	// 创建分表索引
	CreateShardingIndexes()

	zap.S().Info("数据库迁移成功，所有表结构已更新")
	fmt.Println("mysql init successfully")
}

func InitRedis() {
	opt := redis.Options{
		Addr:     fmt.Sprintf("%s:%d", global.ServiceConfig.RedisDB.Host, global.ServiceConfig.RedisDB.Port), // redis地址
		Password: "",                                                                                         // redis密码，没有则留空
		DB:       10,                                                                                         // 默认数据库，默认是0
	}
	global.RedisDB = redis.NewClient(&opt)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	response, err := global.RedisDB.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("connect redis failed: %v", err))
	}

	fmt.Printf("redis init successfully response %v \n", response)

	// 初始化依赖Redis的包
	cache.GetRedisCache()
}
