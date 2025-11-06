package rpc

import (
	"context"
	"fmt"
	"net"

	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// InitGRPC 初始化gRPC服务器
func InitGRPC() *grpc.Server {
	// 创建gRPC服务器
	s := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor),
		grpc.StreamInterceptor(streamInterceptor),
	)

	// 注册服务
	RegisterService(s)

	// 启用反射（用于grpcurl等工具）
	reflection.Register(s)

	zap.S().Info("gRPC服务器初始化完成")
	return s
}

// StartGRPCServer 启动gRPC服务器
func StartGRPCServer(s *grpc.Server, port int) {
	// 监听端口
	addr := fmt.Sprintf(":%d", port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		zap.S().Error("gRPC服务器监听失败", zap.Error(err))
		panic(err)
	}

	zap.S().Infof("gRPC服务器启动在端口 %d", port)

	// 启动服务器
	if err := s.Serve(lis); err != nil {
		zap.S().Error("gRPC服务器启动失败", zap.Error(err))
		panic(err)
	}
}

// unaryInterceptor 一元RPC拦截器（用于日志和错误处理）
func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	zap.S().Debugf("gRPC请求: %s", info.FullMethod)

	resp, err := handler(ctx, req)
	if err != nil {
		zap.S().Errorf("gRPC请求失败: %s, error: %v", info.FullMethod, err)
	} else {
		zap.S().Debugf("gRPC请求成功: %s", info.FullMethod)
	}

	return resp, err
}

// streamInterceptor 流式RPC拦截器（用于日志和错误处理）
func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	zap.S().Debugf("gRPC流请求: %s", info.FullMethod)

	err := handler(srv, ss)
	if err != nil {
		zap.S().Errorf("gRPC流请求失败: %s, error: %v", info.FullMethod, err)
	} else {
		zap.S().Debugf("gRPC流请求完成: %s", info.FullMethod)
	}

	return err
}
