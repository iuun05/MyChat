# MyChat

## 概况

参考的项目地址：[《从0到1搭建一个IM项目》](https://learnku.com/articles/74274)

相关技术栈：Go、Gin、Websocket、UDP、Mysql、Redis、Viper、Gorm、Zap、Md5、Jwt

主要功能

- 登录、注册、用户信息更新、账号注销
- 单聊、群聊
- 发送文字、表情包、图片、语音
- 加好友、好友列表、建群、加入群

系统架构
![alt text](README/DrXEOv9xpl.png)

通信流程
![alt text](README/zDGWUKX9St.png)

项目目录

``` tree
.
├── asset       // 放置上传的图片
├── cache       // redis
├── common      // 放置公共文件
├── config      // 配置文件
├── dao         // MySQL
├── global      // 放置各种连接池，配置等
├── initialize  // 项目初始化
├── middlewear  // 中间件（拦截器）
├── models      // 数据库表设计
├── router      // 路由
├── service     // 对外 api
```

## 初始化

1. 首先创建数据库：

    ```sql
    create database MyChat
    ```

2. 初始化 mod 文件

    ```bash
    go mod init MyChat && go mod tidy
    ```

3. 配置 `/config/config.yaml`

    ```yaml
    port: '8000'
    mysql:
        host: '127.0.0.1'
        port: '3306'
        name: 'MyChat'
        user: 'root'
        password: ''
    redis:
        host: '127.0.0.1'
        port: '6379'
    ```

4. 运行 `main.go`

    ```bash
    go run main.go
    ```

    或者

    ```bash
    go build -o MyChat . && ./MyChat
    ```

最后成功运行就会出现：

``` log
2025-08-26T20:08:10.309+0800    INFO    initialize/config.go:25 配置信息&{8000 {127.0.0.1 3306 MyChat root } {127.0.0.1 6379}}
[GIN-debug] [WARNING] Creating an Engine instance with the Logger and Recovery middleware already attached.

[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:   export GIN_MODE=release
 - using code:  gin.SetMode(gin.ReleaseMode)

[GIN-debug] GET    /v1/user/list             --> MyChat/service.List (4 handlers)
[GIN-debug] POST   /v1/user/login_pw         --> MyChat/service.LoginByNameAndPassWord (3 handlers)
[GIN-debug] POST   /v1/user/new              --> MyChat/service.NewUser (3 handlers)
[GIN-debug] DELETE /v1/user/delete           --> MyChat/service.DeleteUser (4 handlers)
[GIN-debug] POST   /v1/user/updata           --> MyChat/service.UpdataUser (4 handlers)
[GIN-debug] GET    /v1/user/SendUserMsg      --> MyChat/service.SendUserMsg (4 handlers)
[GIN-debug] POST   /v1/relation/list         --> MyChat/service.FriendList (4 handlers)
[GIN-debug] POST   /v1/relation/add          --> MyChat/service.AddFriendByName (4 handlers)
[GIN-debug] POST   /v1/relation/new_group    --> MyChat/service.NewGroup (4 handlers)
[GIN-debug] POST   /v1/relation/group_list   --> MyChat/service.GroupList (4 handlers)
[GIN-debug] POST   /v1/relation/join_group   --> MyChat/service.JoinGroup (4 handlers)
[GIN-debug] POST   /v1/upload/image          --> MyChat/service.Image (3 handlers)
[GIN-debug] POST   /v1/user/redisMsg         --> MyChat/service.RedisMsg (3 handlers)
[GIN-debug] [WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.
Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.
[GIN-debug] Listening and serving HTTP on :8080
```

就可以直接访问服务了，具体的搭建请参考上面的博客。


## 新增功能

1. 获取未读消息功能，并且标记为已读
2. 检查用户在线情况
3. 查询历史记录
