package router

import (
	"MyChat/middlewear"
	"MyChat/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Router() *gin.Engine {
	//初始化路由
	// 使用gin.New()代替gin.Default()，以便自定义Recovery中间件
	router := gin.New()

	// 自定义Recovery中间件，不输出堆栈信息
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// 只记录错误，不输出堆栈
		zap.S().Errorf("panic recovered: %v", recovered)
		c.AbortWithStatus(500)
	}))

	// 使用自定义Logger中间件（可选，如果不需要可以注释掉）
	router.Use(gin.Logger())

	// 健康检查端点
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "MyChat service is running",
		})
	})

	//v1版本
	v1 := router.Group("v1")

	//用户模块，后续有个用户的api就放置其中
	// test

	user := v1.Group("user")
	{
		user.GET("/list", service.List)
		user.POST("/login_pw", service.LoginByNameAndPassWord)
		user.POST("/logout", middlewear.JWY(), service.Logout)

		user.POST("/new", service.NewUser)
		user.POST("/delete", middlewear.JWY(), service.DeleteUser)
		user.POST("/update", middlewear.JWY(), service.UpdataUser)

		user.GET("/SendUserMsg", middlewear.JWY(), service.SendUserMsg)

		// 新增的缓存相关接口
		user.POST("/status", middlewear.JWY(), service.GetUserStatus)
		user.POST("/unread_count", middlewear.JWY(), service.GetUnreadMessageCount)
		user.POST("/mark_read", middlewear.JWY(), service.MarkMessagesAsRead)
		user.POST("/clear_unread", middlewear.JWY(), service.MarkMessagesAsRead)
		user.POST("/history", middlewear.JWY(), service.RedisMsg)
		user.POST("/recent", middlewear.JWY(), service.GetRecentMessages)
	}

	//好友关系
	relation := v1.Group("relation").Use(middlewear.JWY())
	{
		relation.POST("/list", service.FriendList)
		relation.POST("/add", service.AddFriendByName)
		relation.POST("/new_group", service.NewGroup)
		relation.POST("/group_list", service.GroupList)
		relation.POST("/join_group", service.JoinGroup)

		// new
		relation.POST("/online_friends", service.GetOnlineFriends)
		relation.POST("/remove", service.RemoveFriend)
	}

	// 文件传输模块
	upload := v1.Group("upload")
	{
		upload.POST("/image", service.Image)
	}

	//聊天记录
	// v1.POST("/user/redisMsg", service.RedisMsg).Use(middlewear.JWY())

	// 消息模块
	// message := v1.Group("message").Use(middlewear.JWY())
	// {
	// message.POST("/history", service.RedisMsg)
	// message.POST("/recent", service.GetRecentMessages)
	// message.GET("/unread/:userId", service.GetUnreadMessageCount)
	// message.POST("/clear_unread", service.MarkMessagesAsRead)
	// }

	// 聊天记录
	// v1.POST("/user/redisMsg", service.RedisMsg).Use(middlewear.JWY())

	return router
}
