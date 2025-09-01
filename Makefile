# MyChat 项目 Makefile
# 作者: AI Assistant

.PHONY: help test test-simple test-bench test-coverage test-clean install-deps

# 默认目标
help:
	@echo "MyChat 项目构建和测试工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make help          - 显示此帮助信息"
	@echo "  make install-deps  - 安装Go依赖"
	@echo "  make test          - 运行所有测试"
	@echo "  make test-simple   - 运行简化测试"
	@echo "  make test-bench    - 运行基准测试"
	@echo "  make test-coverage - 生成测试覆盖率报告"
	@echo "  make test-clean    - 清理测试数据"
	@echo "  make build         - 构建项目"
	@echo "  make run           - 运行项目"

# 安装依赖
install-deps:
	@echo "📦 安装Go依赖..."
	go mod tidy
	go mod download

# 运行所有测试
test: install-deps
	@echo "🧪 运行所有Redis缓存测试..."
	go test -v ./cache/...

# 运行简化测试
test-simple: install-deps
	@echo "🔍 运行简化测试..."
	go test -v -run TestRedisCache_Simple ./cache/

# 运行基准测试
test-bench: install-deps
	@echo "⚡ 运行性能基准测试..."
	go test -bench=. -benchmem ./cache/

# 生成测试覆盖率报告
test-coverage: install-deps
	@echo "📊 生成测试覆盖率报告..."
	go test -coverprofile=coverage.out ./cache/
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告已生成: coverage.html"

# 清理测试数据
test-clean:
	@echo "🧹 清理测试数据..."
	@rm -f coverage.out coverage.html
	@echo "✅ 清理完成"

# 构建项目
build: install-deps
	@echo "🔨 构建项目..."
	go build -o MyChat .

# 运行项目
run: build
	@echo "🚀 运行项目..."
	./MyChat

# 检查代码质量
lint:
	@echo "🔍 检查代码质量..."
	golangci-lint run

# 格式化代码
fmt:
	@echo "✨ 格式化代码..."
	go fmt ./...
	go vet ./...

# 完整测试套件
test-all: fmt lint test test-bench test-coverage
	@echo "🎉 所有测试完成！"

# 快速验证
quick: test-simple
	@echo "✅ 快速验证完成！"
