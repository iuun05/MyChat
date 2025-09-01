package router

import (
	"MyChat/middlewear"
	"MyChat/service"

	"github.com/gin-gonic/gin"
)

func Router() *gin.Engine {
	//初始化路由
	router := gin.Default()

	//v1版本
	v1 := router.Group("v1")

	//用户模块，后续有个用户的api就放置其中
	// test

	user := v1.Group("user")
	{
		user.GET("/list", service.List)
		user.POST("/login_pw", service.LoginByNameAndPassWord)
		user.POST("/logout", middlewear.JWY(), service.Logout)
		// curl -X POST http://localhost:8080/v1/user/new \
		//  -d "name=testuser" \
		//  -d "password=123456" \
		//  -d "Identity=123456"

		user.POST("/new", service.NewUser)
		user.POST("/delete", middlewear.JWY(), service.DeleteUser)
		user.POST("/update", middlewear.JWY(), service.UpdataUser)
		// wscat -c "ws://localhost:8080/v1/user/SendUserMsg?userId=1"
		// wscat -c "ws://localhost:8080/v1/user/SendUserMsg?userId=2"
		// {"userId":1,"targetId":2,"type":1,"content":"hello user2"}
		user.GET("/SendUserMsg", middlewear.JWY(), service.SendUserMsg)

		// 新增的缓存相关接口
		user.GET("/status", middlewear.JWY(), service.GetUserStatus)
		user.GET("/unread_count", middlewear.JWY(), service.GetUnreadMessageCount)
		user.POST("/mark_read", middlewear.JWY(), service.MarkMessagesAsRead)
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
		relation.GET("/online_friends", service.GetOnlineFriends)
		relation.DELETE("/remove", service.RemoveFriend)
	}

	// 文件传输模块
	upload := v1.Group("upload")
	{
		upload.POST("/image", service.Image)
	}

	//聊天记录
	v1.POST("/user/redisMsg", service.RedisMsg).Use(middlewear.JWY())

	// 消息模块
	message := v1.Group("message").Use(middlewear.JWY())
	{
		message.POST("/history", service.RedisMsg)
		message.POST("/recent", service.GetRecentMessages)
		message.GET("/unread/:userId", service.GetUnreadMessageCount)
		message.POST("/clear_unread", service.MarkMessagesAsRead)
	}

	// 聊天记录
	// v1.POST("/user/redisMsg", service.RedisMsg).Use(middlewear.JWY())

	return router
}
