package main

import (
	"context"
	"fmt"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()
	redisDb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // redis地址
		Password: "",               // redis密码，没有则留空
		DB:       0,                // 默认数据库，默认是0
	})

	// 检查连接
	if _, err := redisDb.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	fmt.Println("redis connected")
}
