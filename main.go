package main

import (
	"fmt"
	"sync"

	"MyChat/global"
	"MyChat/initialize"
	"MyChat/router"
	"MyChat/rpc"

	"go.uber.org/zap"
)

func main() {
	// init logger
	initialize.InitLogger()

	// init config
	initialize.InitConfig()

	// init mysql
	initialize.InitDB()

	initialize.InitRedis()

	// 初始化gRPC服务器
	grpcServer := rpc.InitGRPC()

	// 使用WaitGroup等待两个服务器
	var wg sync.WaitGroup
	wg.Add(2)

	// 启动HTTP服务器（Gin）
	go func() {
		defer wg.Done()
		router := router.Router()
		port := global.ServiceConfig.Port
		if port == 0 {
			port = 8080
		}
		zap.S().Infof("HTTP服务器启动在端口 %d", port)
		if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
			zap.S().Error("HTTP服务器启动失败", zap.Error(err))
		}
	}()

	// 启动gRPC服务器
	go func() {
		defer wg.Done()
		grpcPort := global.ServiceConfig.GRPCPort
		if grpcPort == 0 {
			grpcPort = 50051
		}
		rpc.StartGRPCServer(grpcServer, grpcPort)
	}()

	// 等待所有服务器
	wg.Wait()
}
