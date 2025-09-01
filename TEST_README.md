# MyChat Redis缓存测试文档

## 📋 概述

本文档详细说明了MyChat项目中Redis缓存模块的完整测试功能。测试覆盖了所有缓存操作，包括用户缓存、好友列表缓存、在线状态缓存、群组缓存等核心功能。

## 🚀 快速开始

### 前置条件

1. **Go环境**: Go 1.16+
2. **Redis服务**: Redis 6.0+
3. **依赖包**: testify (自动安装)

### 运行测试

```bash
# 方法1: 使用测试脚本（推荐）
./run_tests.sh

# 方法2: 手动运行
go test -v ./cache/

# 方法3: 运行特定测试
go test -v -run TestRedisCache_UserOperations ./cache/

# 方法4: 运行基准测试
go test -bench=. ./cache/
```

## 🧪 测试功能详解

### 1. 基础功能测试

#### `TestRedisCache_BasicSetAndGet`
- **功能**: 测试基础的设置和获取操作
- **验证**: 确保SetTest和GetTest方法正常工作
- **数据**: 使用用户ID 12345进行测试

### 2. 用户缓存测试

#### `TestRedisCache_UserOperations`
- **功能**: 测试用户缓存的完整生命周期
- **操作**: 
  - 创建测试用户
  - 设置用户缓存
  - 获取用户缓存
  - 验证数据完整性
  - 删除用户缓存
  - 验证删除成功
- **验证点**: 用户名、邮箱、电话等字段正确性

#### `TestRedisCache_GetUserByName`
- **功能**: 测试通过用户名获取用户信息
- **操作**: 设置用户缓存后通过用户名查询
- **验证点**: 用户ID和名称匹配

### 3. 好友列表缓存测试

#### `TestRedisCache_FriendsListOperations`
- **功能**: 测试好友列表的缓存操作
- **操作**:
  - 设置好友列表（3个好友）
  - 获取好友列表
  - 验证好友数量和名称
  - 删除好友列表
  - 验证删除成功
- **验证点**: 好友数量、名称顺序、数据完整性

### 4. 在线状态缓存测试

#### `TestRedisCache_OnlineStatusOperations`
- **功能**: 测试用户在线状态的缓存管理
- **操作**:
  - 设置用户在线状态
  - 检查用户是否在线
  - 设置用户离线
  - 验证离线状态
- **验证点**: 在线状态切换的正确性

### 5. 群组缓存测试

#### `TestRedisCache_CommunityOperations`
- **功能**: 测试群组成员的缓存管理
- **操作**:
  - 设置群组成员ID列表
  - 获取群组成员
  - 验证成员数量和ID
- **验证点**: 成员数量、ID包含关系

### 6. 通用缓存测试

#### `TestRedisCache_GenericOperations`
- **功能**: 测试通用的缓存操作方法
- **操作**:
  - 设置复杂数据结构（map）
  - 获取并验证数据
  - 删除缓存
  - 验证删除成功
- **验证点**: 复杂数据结构的序列化和反序列化

### 7. 缓存过期测试

#### `TestRedisCache_Expiration`
- **功能**: 测试缓存的过期机制
- **操作**:
  - 设置短期缓存（1秒）
  - 立即获取验证成功
  - 等待过期
  - 验证过期后获取失败
- **验证点**: 过期时间的准确性

### 8. 并发操作测试

#### `TestRedisCache_ConcurrentOperations`
- **功能**: 测试高并发场景下的缓存操作
- **操作**:
  - 10个goroutine并发设置缓存
  - 每个goroutine执行100次操作
  - 验证所有操作成功
- **验证点**: 并发安全性、数据一致性

### 9. 错误情况测试

#### `TestRedisCache_ErrorCases`
- **功能**: 测试边界情况和错误处理
- **操作**:
  - 设置nil值
  - 获取nil值
  - 删除不存在的key
- **验证点**: 错误处理的健壮性

### 10. 性能基准测试

#### `BenchmarkRedisCache_SetUser`
- **功能**: 测试用户缓存设置的性能
- **指标**: 每秒可执行的SetUser操作数

#### `BenchmarkRedisCache_GetUser`
- **功能**: 测试用户缓存获取的性能
- **指标**: 每秒可执行的GetUser操作数

### 11. 数据清理测试

#### `TestRedisCache_Cleanup`
- **功能**: 测试测试数据的清理
- **操作**: 清空测试数据库并验证
- **验证点**: 清理操作的完整性

## 🔧 测试配置

### Redis配置
- **地址**: 127.0.0.1:6379
- **数据库**: DB 1 (避免影响生产数据)
- **连接**: 自动连接和断开

### 测试数据
- **用户ID范围**: 1001-9999
- **好友数量**: 3个
- **群组成员**: 4个
- **缓存过期时间**: 1秒到60分钟不等

## 📊 测试覆盖率

运行测试覆盖率分析：
```bash
go test -coverprofile=coverage.out ./cache/
go tool cover -html=coverage.out -o coverage.html
```

## 🐛 常见问题

### 1. Redis连接失败
**错误**: `无法连接到Redis`
**解决**: 确保Redis服务正在运行
```bash
sudo systemctl start redis
# 或
redis-server
```

### 2. 测试数据库冲突
**错误**: 测试数据影响其他环境
**解决**: 测试使用独立的DB 1，自动清理

### 3. 依赖包缺失
**错误**: `cannot find package`
**解决**: 运行 `go mod tidy` 安装依赖

## 📈 性能指标

### 基准测试结果示例
```
BenchmarkRedisCache_SetUser-8     10000    123456 ns/op
BenchmarkRedisCache_GetUser-8     20000     98765 ns/op
```

### 性能优化建议
1. 使用连接池管理Redis连接
2. 批量操作减少网络往返
3. 合理设置缓存过期时间
4. 监控内存使用情况

## 🔍 调试技巧

### 1. 查看测试日志
```bash
go test -v -logtostderr ./cache/
```

### 2. 运行单个测试
```bash
go test -v -run TestRedisCache_UserOperations ./cache/
```

### 3. 设置测试超时
```bash
go test -timeout 30s ./cache/
```

### 4. 并行测试
```bash
go test -parallel 4 ./cache/
```

## 📝 测试维护

### 添加新测试
1. 在`redis_cache_test.go`中添加新测试函数
2. 遵循命名规范：`TestRedisCache_功能名称`
3. 使用`require`和`assert`进行断言
4. 添加适当的注释说明

### 更新测试数据
1. 修改`createTestUser`函数
2. 更新测试用例中的期望值
3. 确保测试数据的合理性

### 测试环境隔离
1. 使用独立的Redis数据库
2. 测试前后自动清理数据
3. 避免影响生产环境

## 🤝 贡献指南

1. **代码规范**: 遵循Go官方代码规范
2. **测试覆盖**: 新功能必须包含测试
3. **文档更新**: 同步更新测试文档
4. **性能考虑**: 关注测试执行时间

## 📞 技术支持

如有问题，请：
1. 查看本文档的常见问题部分
2. 检查Redis服务状态
3. 查看测试日志输出
4. 联系开发团队

---

**最后更新**: $(date)
**版本**: 1.0.0
**维护者**: AI Assistant
