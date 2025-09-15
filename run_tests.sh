#!/bin/bash

# MyChat 完整测试运行脚本
# 作者: AI Assistant
# 日期: $(date)

echo "🚀 开始运行MyChat完整测试套件..."

# 检查Redis是否运行
echo "📋 检查Redis服务状态..."
if ! redis-cli ping > /dev/null 2>&1; then
    echo "❌ Redis服务未运行，请先启动Redis服务"
    echo "   启动命令: sudo systemctl start redis 或 redis-server"
    exit 1
fi
echo "✅ Redis服务运行正常"

# 检查Go环境
echo "📋 检查Go环境..."
if ! command -v go &> /dev/null; then
    echo "❌ Go未安装，请先安装Go"
    exit 1
fi
echo "✅ Go环境正常"

# 安装依赖
echo "📦 安装Go依赖..."
go mod tidy

# 创建测试结果目录
mkdir -p test_results

# ==================== 1. 缓存测试 ====================
echo ""
echo "🧪 1. 运行Redis缓存测试..."
echo "----------------------------------------"
if go test -v ./cache/... -timeout 30s > test_results/cache_test.log 2>&1; then
    echo "✅ 缓存测试通过"
    cat test_results/cache_test.log | grep -E "(PASS|FAIL|RUN)"
else
    echo "❌ 缓存测试失败"
    cat test_results/cache_test.log | grep -E "(FAIL|ERROR)"
fi

# ==================== 2. Message测试 ====================
echo ""
echo "🌐 2. 运行Message功能测试..."
echo "----------------------------------------"
if go test -v ./dao/ -run "TestBroMsg|TestSendMsg|TestSendMsgAndSave|TestGetRecentMessages|TestClearUnreadCount|TestGetUnreadCount|TestReadRedisMsg|TestDispatch|TestSendGroupMsg|TestGetUnreadMsg" -timeout 60s > test_results/message_basic_test.log 2>&1; then
    echo "✅ Message基础功能测试通过"
    cat test_results/message_basic_test.log | grep -E "(PASS|FAIL|RUN)" | tail -10
else
    echo "❌ Message基础功能测试失败"
    cat test_results/message_basic_test.log | grep -E "(FAIL|ERROR)" | tail -5
fi

# ==================== 3. WebSocket测试 ====================
echo ""
echo "🔌 3. 运行WebSocket测试..."
echo "----------------------------------------"
if go test -v ./dao/ -run "TestWebSocket" -timeout 60s > test_results/websocket_test.log 2>&1; then
    echo "✅ WebSocket测试通过"
    cat test_results/websocket_test.log | grep -E "(PASS|FAIL|RUN)" | tail -10
else
    echo "❌ WebSocket测试失败"
    cat test_results/websocket_test.log | grep -E "(FAIL|ERROR)" | tail -5
fi

# ==================== 4. UDP测试 ====================
echo ""
echo "📡 4. 运行UDP测试..."
echo "----------------------------------------"
if go test -v ./dao/ -run "TestUdp" -timeout 30s > test_results/udp_test.log 2>&1; then
    echo "✅ UDP测试通过"
    cat test_results/udp_test.log | grep -E "(PASS|FAIL|RUN)"
else
    echo "❌ UDP测试失败"
    cat test_results/udp_test.log | grep -E "(FAIL|ERROR)" | tail -5
fi

# ==================== 5. 协程测试 ====================
echo ""
echo "⚙️ 5. 运行协程测试..."
echo "----------------------------------------"
if go test -v ./dao/ -run "TestSendProc|TestRecProc" -timeout 30s > test_results/goroutine_test.log 2>&1; then
    echo "✅ 协程测试通过"
    cat test_results/goroutine_test.log | grep -E "(PASS|FAIL|RUN)"
else
    echo "❌ 协程测试失败"
    cat test_results/goroutine_test.log | grep -E "(FAIL|ERROR)" | tail -5
fi

# ==================== 6. 集成测试 ====================
echo ""
echo "🔗 6. 运行集成测试..."
echo "----------------------------------------"
if go test -v ./dao/ -run "TestMessageIntegration|TestMessageErrorHandling|TestMessageCleanup" -timeout 60s > test_results/integration_test.log 2>&1; then
    echo "✅ 集成测试通过"
    cat test_results/integration_test.log | grep -E "(PASS|FAIL|RUN)"
else
    echo "❌ 集成测试失败"
    cat test_results/integration_test.log | grep -E "(FAIL|ERROR)" | tail -5
fi

# ==================== 7. 性能基准测试 ====================
echo ""
echo "⚡ 7. 运行性能基准测试..."
echo "----------------------------------------"
echo "运行缓存性能测试..."
go test -bench=BenchmarkRedis -benchmem ./cache/... -timeout 30s > test_results/cache_benchmark.log 2>&1

echo "运行Message性能测试..."
go test -bench=BenchmarkSendMsg -benchmem ./dao/ -timeout 30s > test_results/message_benchmark.log 2>&1

echo "运行WebSocket性能测试..."
go test -bench=BenchmarkWebSocket -benchmem ./dao/ -timeout 30s > test_results/websocket_benchmark.log 2>&1

# 显示性能测试结果
echo "📊 性能测试结果:"
echo "缓存性能:"
cat test_results/cache_benchmark.log | grep -E "Benchmark|ns/op|B/op|allocs/op" | head -5
echo "Message性能:"
cat test_results/message_benchmark.log | grep -E "Benchmark|ns/op|B/op|allocs/op" | head -5
echo "WebSocket性能:"
cat test_results/websocket_benchmark.log | grep -E "Benchmark|ns/op|B/op|allocs/op" | head -5

# ==================== 8. 生成覆盖率报告 ====================
echo ""
echo "📊 8. 生成测试覆盖率报告..."
echo "----------------------------------------"

# 缓存覆盖率
echo "生成缓存测试覆盖率..."
go test -coverprofile=test_results/cache_coverage.out ./cache/... -timeout 30s
go tool cover -html=test_results/cache_coverage.out -o test_results/cache_coverage.html

# Message覆盖率
echo "生成Message测试覆盖率..."
go test -coverprofile=test_results/message_coverage.out ./dao/ -timeout 60s
go tool cover -html=test_results/message_coverage.out -o test_results/message_coverage.html

# 整体覆盖率
echo "生成整体测试覆盖率..."
go test -coverprofile=test_results/overall_coverage.out ./... -timeout 120s
go tool cover -html=test_results/overall_coverage.out -o test_results/overall_coverage.html

# ==================== 9. 测试总结 ====================
echo ""
echo "🎉 测试完成！"
echo "========================================"
echo "📁 测试结果文件:"
echo "   - test_results/cache_test.log - 缓存测试日志"
echo "   - test_results/message_basic_test.log - Message基础功能测试日志"
echo "   - test_results/websocket_test.log - WebSocket测试日志"
echo "   - test_results/udp_test.log - UDP测试日志"
echo "   - test_results/goroutine_test.log - 协程测试日志"
echo "   - test_results/integration_test.log - 集成测试日志"
echo "   - test_results/cache_benchmark.log - 缓存性能测试日志"
echo "   - test_results/message_benchmark.log - Message性能测试日志"
echo "   - test_results/websocket_benchmark.log - WebSocket性能测试日志"
echo ""
echo "📊 覆盖率报告:"
echo "   - test_results/cache_coverage.html - 缓存测试覆盖率"
echo "   - test_results/message_coverage.html - Message测试覆盖率"
echo "   - test_results/overall_coverage.html - 整体测试覆盖率"
echo ""
echo "🔍 查看详细结果:"
echo "   cat test_results/*.log | grep -E '(PASS|FAIL)' | sort | uniq -c"
echo "   open test_results/overall_coverage.html"
echo ""
echo "✅ 所有测试已完成！"
