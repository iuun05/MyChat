package service

import (
	"MyChat/cache"
	"MyChat/common"
	"MyChat/dao"
	"MyChat/global"
	"MyChat/middlewear"
	"MyChat/models"
	"fmt"
	"math/rand"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	redisCache *cache.RedisCache
	userDAO    *dao.UserDAO
	messageDAO *dao.MessageDAO
)

func Init() {
	if global.RedisDB != nil && redisCache == nil {
		redisCache = cache.NewRedisCache()
	} else {
		zap.S().Error("global redis db is nil")
	}

	if userDAO == nil {
		userDAO = dao.NewUserDAO()
	}

	if messageDAO == nil {
		messageDAO = dao.NewMessageDAO()
	}
}

// getRedisCache 安全地获取Redis缓存实例
func getRedisCache() *cache.RedisCache {
	if redisCache == nil {
		Init()
		if redisCache == nil {
			zap.S().Warn("Redis缓存未初始化，Redis连接可能有问题")
			return nil
		}
	}
	return redisCache
}

func getUserDAO() *dao.UserDAO {
	if userDAO == nil {
		Init()
	}
	return userDAO
}

func InitUserDAO() {
	if userDAO == nil {
		Init()
	}
}

// List 获取用户列表
func List(ctx *gin.Context) {
	list, err := getUserDAO().GetUserList()
	if err != nil {
		common.Error(ctx, -1, "获取用户列表失败")
		return
	}
	common.Success(ctx, list, "获取用户列表成功")
}

// LoginByNameAndPassWord 登录
func LoginByNameAndPassWord(ctx *gin.Context) {
	var req common.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	data, err := getUserDAO().FindUserByName(req.Name)
	if err != nil {
		common.Error(ctx, -1, "登录失败")
		return
	}

	if data.Name == "" {
		common.Error(ctx, -1, "用户名不存在")
		return
	}

	ok := common.CheckPassWord(req.Password, data.Salt, data.PassWord)
	if !ok {
		common.Error(ctx, -1, "密码错误")
		return
	}

	Rsp, err := getUserDAO().FindUserByNameAndPwd(req.Name, data.PassWord)
	if err != nil {
		zap.S().Info("登录失败", err)
		common.Error(ctx, -1, "登录失败")
		return
	}

	token, err := middlewear.GenerateToken(Rsp.ID, "yk")
	if err != nil {
		zap.S().Info("生成token失败", err)
		common.InternalError(ctx, "生成token失败")
		return
	}

	if err := getRedisCache().SetUserOnline(Rsp.ID, "online"); err != nil {
		zap.S().Warn("[LoginByNameAndPassWord/service/user] Failed to set user online status ", err)
	}

	common.Success(ctx, gin.H{
		"tokens": token,
		"userId": Rsp.ID,
	}, "登录成功")
}

// GetUserStatus 获取用户在线状态
func GetUserStatus(ctx *gin.Context) {
	var req common.GetUserStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	isOnline, err := getRedisCache().IsUserOnline(uint(req.UserId))
	if err != nil {
		zap.S().Error("[GetUserStatus/service/user] Failed to check user online status ", err)
		common.InternalError(ctx, "服务器错误")
		return
	}

	common.Success(ctx, gin.H{
		"isOnline": isOnline,
	}, "success")
}

// Logout 用户登出
func Logout(ctx *gin.Context) {
	var req common.BaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if err := getRedisCache().SetUserOffline(uint(req.UserId)); err != nil {
		zap.S().Warn("[Logout/service/user] Failed to set user offline status", err)
	}

	common.Success(ctx, nil, "登出成功")
}

// GetUnreadMessageCount 获取未读消息数
func GetUnreadMessageCount(ctx *gin.Context) {
	var req common.GetUnreadCountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	count, err := messageDAO.GetUnreadCount(ctx.Request.Context(), req.UserId)
	if err != nil {
		zap.S().Error("[GetUnreadMessageCount/service/user] Failed to get the number of unread messages ", err)
		common.InternalError(ctx, "服务器错误")
		return
	}

	common.Success(ctx, gin.H{
		"count": count,
	}, "success")
}

// MarkMessagesAsRead 标记消息为已读
func MarkMessagesAsRead(ctx *gin.Context) {
	var req common.MarkReadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	err := messageDAO.ClearUnreadCount(ctx.Request.Context(), req.UserId)
	if err != nil {
		zap.S().Error("[MarkMessagesAsRead/service/user] Failed to clear the number of unread messages", err)
		common.InternalError(ctx, "服务器错误")
		return
	}

	common.Success(ctx, nil, "已标记为已读")
}

// NewUser 注册新用户
func NewUser(ctx *gin.Context) {
	var req common.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if req.Name == "" || req.Password == "" || req.Repassword == "" {
		common.Error(ctx, -1, "用户名或密码不能为空！")
		return
	}

	_, err := getUserDAO().FindUser(req.Name)
	if err != nil {
		common.ErrorWithData(ctx, -1, err.Error(), nil)
		return
	}

	if req.Password != req.Repassword {
		common.Error(ctx, -1, "两次密码不一致！")
		return
	}

	user := models.UserBasic{}
	salt := fmt.Sprintf("%d", rand.Int31())
	user.Name = req.Name
	user.PassWord = common.SaltPassWord(req.Password, salt)
	user.Salt = salt
	t := time.Now()
	user.LoginTime = &t
	user.LoginOutTime = &t
	user.HeartBeatTime = &t

	getUserDAO().CreateUser(user)
	common.Success(ctx, user, "新增用户成功！")
}

// UpdataUser 更新用户信息
func UpdataUser(ctx *gin.Context) {
	var req common.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	user := models.UserBasic{}
	user.ID = uint(req.UserId)

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Password != "" {
		salt := fmt.Sprintf("%d", rand.Int31())
		user.Salt = salt
		user.PassWord = common.SaltPassWord(req.Password, salt)
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Gender != "" {
		user.Gender = req.Gender
	}

	_, err := govalidator.ValidateStruct(user)
	if err != nil {
		zap.S().Info("参数不匹配", err)
		common.BadRequest(ctx, "参数不匹配")
		return
	}

	Rsp, err := getUserDAO().UpdateUser(user)
	if err != nil {
		zap.S().Info("更新用户失败", err)
		common.InternalError(ctx, "修改信息失败")
		return
	}

	common.Success(ctx, gin.H{
		"name": Rsp.Name,
	}, "修改成功")
}

// DeleteUser 删除用户
func DeleteUser(ctx *gin.Context) {
	var req common.DeleteUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	user := models.UserBasic{}
	user.ID = uint(req.UserId)

	err := getUserDAO().DeleteUser(user)
	if err != nil {
		zap.S().Info("注销用户失败", err)
		common.InternalError(ctx, "注销账号失败")
		return
	}

	common.Success(ctx, nil, "注销账号成功")
}

// SendUserMsg WebSocket连接
func SendUserMsg(ctx *gin.Context) {
	dao.Chat(ctx.Writer, ctx.Request)
}
