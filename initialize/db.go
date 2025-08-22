package initialize

import (
	"MyChat/global"
	"fmt"
	"log"
	"os"
	"time"

	redis "github.com/redis/go-redis/v9"
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
		panic(err)
	}
}

func InitRedis() {
	opt := redis.Options{
		Addr:     fmt.Sprintf("%s:%d", global.ServiceConfig.RedisDB.Host, global.ServiceConfig.RedisDB.Port), // redis地址
		Password: "",                                                                                         // redis密码，没有则留空
		DB:       10,                                                                                         // 默认数据库，默认是0
	}
	global.RedisDB = redis.NewClient(&opt)
}
