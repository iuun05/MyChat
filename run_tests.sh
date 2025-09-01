#!/bin/bash

# MyChat Redis缓存测试运行脚本
# 作者: AI Assistant
# 日期: $(date)

echo "🚀 开始运行MyChat Redis缓存测试..."

# 检查Redis是否运行
echo "📋 检查Redis服务状态..."
if ! redis-cli ping > /dev/null 2>&1; then
    echo "❌ Redis服务未运行，请先启动Redis服务"
    echo "   启动命令: sudo systemctl start redis 或 redis-server"
    exit 1
fi
echo "✅ Redis服务运行正常"

# 安装依赖
echo "📦 安装Go依赖..."
go mod tidy

# 运行所有测试
echo "🧪 运行Redis缓存测试..."
go test -v ./cache/...

# 运行基准测试
echo "⚡ 运行性能基准测试..."
go test -bench=. ./cache/...

# 运行测试覆盖率
echo "📊 生成测试覆盖率报告..."
go test -coverprofile=coverage.out ./cache/...
go tool cover -html=coverage.out -o coverage.html

echo "🎉 测试完成！"
echo "📁 测试覆盖率报告已生成: coverage.html"
echo "📁 覆盖率数据文件: coverage.out"
