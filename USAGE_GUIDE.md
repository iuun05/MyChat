# MyChat 用户使用指南

## 快速开始

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

### 环境配置

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

### 获取用户状态

```bash
curl -X POST "http://localhost:8080/v1/user/status" \
        -d "userId=17307" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA3LCJleHAiOjE3NjE5MzE3OTUsImlzcyI6InlrIn0.Lsn1p
QVPqLynjIN6xygMDqFcK3YtJkRzTAMfGml7bSE"
```

**响应结果:**

```json
{"code":0,"isOnline":false,"message":"success"}
```

### 获取未读消息数量 // 标记为已读

```bash
curl -X POST "http://localhost:8080/v1/user/unread_count" \
        -d "userId=17307" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA3LCJleHAiOjE3NjE5MzE3OTUsImlzcyI6InlrIn0.Lsn1p
QVPqLynjIN6xygMDqFcK3YtJkRzTAMfGml7bSE"
```

```bash
curl -X POST "http://localhost:8080/v1/user/mark_read" \
        -d "userId=17307" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA3LCJleHAiOjE3NjE5MzE3OTUsImlzcyI6InlrIn0.Lsn1p
QVPqLynjIN6xygMDqFcK3YtJkRzTAMfGml7bSE"
```

## 好友系统

### 添加好友

```bash
# 通过用户ID添加好友
curl -X POST http://localhost:8080/v1/relation/add \                         
        -d "userId=17309" \                                                    
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX80" \                                                                            
        -d "targetId=17308"
```

**响应示例:**
```json
{"code":0,"message":"添加好友成功"}
```

### 好友列表查询
```bash
curl -X POST http://localhost:8080/v1/relation/list \
        -d "userId=17309" -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvn
omdabol0BqQvHbrvX80"
```

**响应示例:**

```json
{"Code":0,"Msg":"","Data":[{"Name":"testuser22","Avatar":"","Gender":"male","Phone":"","Email":"newemail@example.com","Identity":"8ed6e7251605894295e5717513c57c2f"}],"Rows":null,"Total":1}
```

### 删除好友

```bash
curl -X POST http://localhost:8080/v1/relation/remove \
        -d "userId=17309" -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX80" \
        -d "targetId=17308"
```

**响应示例:**

```json
{"code":0,"message":"移除好友成功"}
```

## 群组功能

### 创建群组
```bash
curl -X POST http://localhost:8080/v1/relation/new_group \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
        -d "ownerId=17309" \
        -d "cate=2" \
        -d "icon=./asset/upload/17558805971104473141.png" \
        -d "desc=nothing" \
        -d "name=学习交流群"
```

**响应示例:**

```json
{"code":0,"message":"建群成功"}
```

### 加入群组

```bash
curl -X POST http://localhost:8080/v1/relation/join_group \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
        -d "comId=学习交流群"
```

**响应示例:**

```json
{"code":0,"message":"加群成功"}
```

### 群组列表查询

```bash
curl -X POST http://localhost:8080/v1/relation/group_list \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
        -d "ownerId=17309"
```

**响应示例:**

```json
{"Code":0,"Msg":"","Data":[{"ID":8,"CreatedAt":"2025-09-02T00:10:21.982+08:00","UpdatedAt":"2025-09-02T00:10:21.982+08:00","DeletedAt":null,"Name":"Group2","OwnerId":10760,"Type":1,"Image":"","Desc":""},{"ID":15,"CreatedAt":"2025-09-13T22:39:33.817+08:00","UpdatedAt":"2025-09-13T22:39:33.817+08:00","DeletedAt":null,"Name":"学习交流群","OwnerId":17309,"Type":2,"Image":"./asset/upload/17558805971104473141.png","Desc":"nothing"}],"Rows":null,"Total":2}
```

## 在线状态

### 检查用户在线状态

```bash
curl -X POST http://localhost:8080/v1/relation/online_friends \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0"
```

**响应示例:**

```json
{"Code":0,"Msg":"获取在线好友成功","Data":[{"avatar":"","id":17308,"isOnline":false,"name":"testuser22"}],"Rows":null,"Total":null}
```

### 设置用户离线

```bash
curl -X POST http://localhost:8080/v1/user/logout \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0"
```

**响应示例:**

```json
{"code":0,"message":"登出成功"}
```

## 缓存

### 历史记录

```bash
curl -X POST http://localhost:8080/v1/user/redisMsg \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
      -d "userIdA=17309" \
      -d "userIdB=17308" \
      -d "start=1" \
      -d "end=2"
```

```bash
curl -X POST http://localhost:8080/v1/user/history \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
      -d "userIdA=17309" \
      -d "userIdB=17308" \
      -d "start=1" \
      -d "end=2"
```

**响应示例:**

```json
{"Code":0,"Msg":"","Data":"ok","Rows":null,"Total":[]}
```

### 获取缓存记录

```bash
curl -X POST http://localhost:8080/v1/user/history \
        -d "userId=17309" \
        -d "token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOjE3MzA5LCJleHAiOjE3NjI5NTU3MzEsImlzcyI6InlrIn0.DkbyZoY-ZkSHyoVTDVAmJGvnomdabol0BqQvHbrvX8
0" \
      -d "userIdA=17309" \
      -d "userIdB=17308" \
      -d "limit=20"
```

**响应示例:**

```json
{"Code":0,"Msg":"","Data":"ok","Rows":null,"Total":[]}
```

## 配置说明

