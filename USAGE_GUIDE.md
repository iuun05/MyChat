# MyChat 用户使用指南

## 快速开始

### 环境配置

### 安装步骤

```bash
# 1. 克隆项目
git clone https://github.com/iuun05/MyChat.git
cd MyChat

# 2. 安装依赖
go mod tidy

# 3. 配置环境
cp .env.example .env
# 编辑 .env 文件，配置数据库和Redis连接信息

# 4. 启动服务
go run main.go
```

### 服务启动

```bash
go build -o MyChat . && ./MyChat
```

## 用户管理

### 用户注册

```bash
curl -X POST http://localhost:8080/v1/user/new \
       -d "name=testuser22" \
       -d "password=123456" \
       -d "Identity=123456"
```

**响应示例:**

```json
{"code":0,"data":{"ID":0,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"testuser22","PassWord":"e10adc3949ba59abbe56e057f20f883e$323186143","Avatar":"","Gender":"","Phone":"","Email":"","Identity":"","ClientIp":"","ClientPort":"","Salt":"323186143","LoginTime":"2025-09-02T00:46:42.560956541+08:00","HeartBeatTime":"2025-09-02T00:46:42.560956541+08:00","LoginOutTime":"2025-09-02T00:46:42.560956541+08:00","IsLoginOut":false,"DeviceInfo":""},"message":"新增用户成功！"}     
```

### 用户登录

```bash
curl -X POST http://localhost:8080/v1/user/login_pw \
        -d "name=testuser22" \
        -d "password=123456"
```

**响应示例:**

```json
{"code":0,"message":"登录成功","tokens":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA4LCJleHAiOjE3NjE5MzA4OTYsImlzcyI6InlrIn0.6V9czItuA_2MrSgElIbcHW39wLAqqcIcnc27V3_nVH0","userId":17308}
```

### 用户信息查询

```bash
# TODO 全局查询，其他的还没有完成，后续等待完成
curl -X GET http://localhost:8080/v1/user/list
# 通过用户ID查询

# 通过用户名查询
```

### 用户信息更新

```bash
curl -X POST http://localhost:8080/v1/user/update \
        -d "userId=17308" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA4LCJleHAiOjE3NjE5Mjk2OTUsImlzcyI6InlrIn0.66Laf
m6WF5SPmOzkxsHXdaMMIGALVu9wDzG7iTeG1B8" \
        -d "name=newusername" \
        -d "email=newemail@example.com"
```

**响应示例:**

```json
{"code":0,"data":"newusername","message":"修改成功"}
```

### 用户删除

```bash
curl -X POST http://localhost:8080/v1/user/delete \
    -d "userId=17307" \
    -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA3LCJleHAiOjE3NjE5MzE3OTUsImlzcyI6InlrIn0.Lsn1pQVPqLynjIN6xygMDqFcK3YtJkRzTAMfGml7bSE"
```

## 👥 好友系统

### 添加好友
```bash
# 通过用户ID添加好友
curl -X POST http://localhost:8080/v1/friend/add \
  -H "Authorization: Bearer abc123def456" \
  -d "target_id=1002"

# 通过用户名添加好友
curl -X POST http://localhost:8080/v1/friend/add \
  -H "Authorization: Bearer abc123def456" \
  -d "target_name=frienduser"
```

**响应示例:**
```json
{
  "code": 200,
  "message": "添加好友成功",
  "data": {
    "status": 1,
    "message": "好友关系已建立"
  }
}
```

### 好友列表查询
```bash
curl "http://localhost:8080/v1/friend/list" \
  -H "Authorization: Bearer abc123def456"
```

**响应示例:**
```json
{
  "code": 200,
  "message": "获取好友列表成功",
  "data": [
    {
      "id": 1002,
      "name": "frienduser",
      "email": "friend@example.com",
      "avatar": "avatar.jpg"
    }
  ]
}
```

### 删除好友
```bash
curl -X DELETE http://localhost:8080/v1/friend/remove \
  -H "Authorization: Bearer abc123def456" \
  -d "target_id=1002"
```

## 🏘️ 群组功能

### 创建群组
```bash
curl -X POST http://localhost:8080/v1/community/create \
  -H "Authorization: Bearer abc123def456" \
  -d "name=测试群组" \
  -d "description=这是一个测试群组"
```

### 加入群组
```bash
curl -X POST http://localhost:8080/v1/community/join \
  -H "Authorization: Bearer abc123def456" \
  -d "community_id=1"
```

### 群组列表查询
```bash
curl "http://localhost:8080/v1/community/list" \
  -H "Authorization: Bearer abc123def456"
```

### 群组成员查询
```bash
curl "http://localhost:8080/v1/community/members?community_id=1" \
  -H "Authorization: Bearer abc123def456"
```

## 🟢 在线状态

### 检查用户在线状态
```bash
curl "http://localhost:8080/v1/user/online?user_id=1001" \
  -H "Authorization: Bearer abc123def456"
```

**响应示例:**
```json
{
  "code": 200,
  "message": "查询成功",
  "data": {
    "user_id": 1001,
    "is_online": true,
    "node_info": "node1.example.com:8080",
    "last_seen": "2025-09-02T00:00:00Z"
  }
}
```

### 设置用户离线
```bash
curl -X POST http://localhost:8080/v1/user/logout \
  -H "Authorization: Bearer abc123def456"
```

## 💾 缓存系统

### 缓存状态查询
```bash
# 查询Redis连接状态
curl "http://localhost:8080/v1/cache/status"

# 查询缓存统计信息
curl "http://localhost:8080/v1/cache/stats"
```

### 缓存清理
```bash
# 清理用户缓存
curl -X POST http://localhost:8080/v1/cache/clear/user \
  -H "Authorization: Bearer abc123def456" \
  -d "user_id=1001"

# 清理好友列表缓存
curl -X POST http://localhost:8080/v1/cache/clear/friends \
  -H "Authorization: Bearer abc123def456" \
  -d "user_id=1001"
```

## 🔌 API接口

### 基础URL
```
http://localhost:8080/v1
```

### 认证方式
所有需要认证的接口都需要在请求头中包含：
```
Authorization: Bearer <identity_token>
```

### 响应格式
```json
{
  "code": 200,           // 状态码
  "message": "成功",      // 消息
  "data": {},            // 数据
  "timestamp": "2025-09-02T00:00:00Z"  // 时间戳
}
```

### 状态码说明
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未认证
- `403`: 权限不足
- `404`: 资源不存在
- `500`: 服务器内部错误

## ⚙️ 配置说明

### 环境变量
```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=password
DB_NAME=MyChat

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# 服务配置
SERVER_PORT=8080
SERVER_MODE=debug
```

### 配置文件
```yaml
# config/config.yaml
server:
  port: 8080
  mode: debug
  
database:
  host: localhost
  port: 3306
  user: root
  password: password
  name: MyChat
  
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  
cache:
  user_expiration: 60m
  friends_expiration: 15m
  online_expiration: 5m
```
