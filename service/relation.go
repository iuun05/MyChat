package service

import (
	"MyChat/common"
	"MyChat/dao"
	"MyChat/models"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type user struct {
	Name     string
	Avatar   string
	Gender   string
	Phone    string
	Email    string
	Identity string
}

// GetOnlineFriends 获取在线好友列表
func GetOnlineFriends(ctx *gin.Context) {
	var req common.GetOnlineFriendsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	friends, err := dao.FriendList(uint(req.UserId))
	if err != nil {
		common.Error(ctx, -1, "获取好友列表失败")
		return
	}

	onlineFriends := make([]map[string]any, 0)
	for _, friend := range *friends {
		var isOnline bool
		if cache := getRedisCache(); cache != nil {
			isOnline, err = cache.IsUserOnline(friend.ID)
			if err != nil {
				zap.S().Warn("[GetOnlineFriends/service/relation] Failed to check friend online status ", err)
				continue
			}
		}

		friendInfo := map[string]any{
			"id":       friend.ID,
			"name":     friend.Name,
			"avatar":   friend.Avatar,
			"isOnline": isOnline,
		}
		onlineFriends = append(onlineFriends, friendInfo)
	}

	common.Success(ctx, onlineFriends, "获取在线好友成功")
}

// RemoveFriend 移除好友
func RemoveFriend(ctx *gin.Context) {
	var req common.RemoveFriendRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	err := dao.RemoveFriend(uint(req.UserId), uint(req.TargetId))
	if err != nil {
		common.Error(ctx, -1, "移除好友失败")
		return
	}

	common.Success(ctx, nil, "移除好友成功")
}

// GetRecentMessages 获取最近消息
func GetRecentMessages(ctx *gin.Context) {
	var req common.GetRecentMessagesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}

	messages, err := dao.GetRecentMessages(req.UserId, req.UserIdB, req.Limit)
	if err != nil {
		common.Error(ctx, -1, "获取消息失败")
		return
	}

	common.Success(ctx, messages, "获取消息成功")
}

// FriendList 获取好友列表
func FriendList(ctx *gin.Context) {
	var req common.FriendListRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	users, err := dao.FriendList(uint(req.UserId))
	if err != nil {
		zap.S().Info("获取好友列表失败", err)
		common.Error(ctx, -1, "好友为空")
		return
	}

	infos := make([]user, 0)
	for _, v := range *users {
		info := user{
			Name:     v.Name,
			Avatar:   v.Avatar,
			Gender:   v.Gender,
			Phone:    v.Phone,
			Email:    v.Email,
			Identity: v.Identity,
		}
		infos = append(infos, info)
	}

	common.SuccessWithTotal(ctx, infos, len(infos), "获取好友列表成功")
}

// AddFriendByName 添加好友
func AddFriendByName(ctx *gin.Context) {
	var req common.AddFriendRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	var code int
	var err error

	// 判断targetId是数字还是字符串（用户名）
	if req.TargetId != "" {
		code, err = dao.AddFriendByName(uint(req.UserId), req.TargetId)
		if err != nil {
			HandleErr(code, ctx, err)
			return
		}
	} else {
		common.BadRequest(ctx, "targetId不能为空")
		return
	}

	common.Success(ctx, nil, "添加好友成功")
}

// HandleErr 处理错误
func HandleErr(code int, ctx *gin.Context, err error) {
	switch code {
	case -1:
		common.Error(ctx, -1, err.Error())
	case 0:
		common.Error(ctx, -1, "该好友已经存在")
	case -2:
		common.Error(ctx, -1, "不能添加自己")
	default:
		common.Error(ctx, -1, "未知错误")
	}
}

// NewGroup 新建群聊
func NewGroup(ctx *gin.Context) {
	var req common.NewGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if req.UserId == 0 {
		common.Error(ctx, -1, "您未登录")
		return
	}

	if req.Name == "" {
		common.Error(ctx, -1, "群名称不能为空")
		return
	}

	community := models.Community{}
	if req.Icon != "" {
		community.Image = req.Icon
	}
	if req.Desc != "" {
		community.Desc = req.Desc
	}

	community.Name = req.Name
	community.Type = req.Type
	community.OwnerId = uint(req.UserId)

	code, err := dao.CreateCommunity(community)
	if err != nil {
		HandleErr(code, ctx, err)
		return
	}

	common.Success(ctx, nil, "建群成功")
}

// GroupList 获取群列表
func GroupList(ctx *gin.Context) {
	var req common.GroupListRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if req.UserId == 0 {
		common.Error(ctx, -1, "您未登录")
		return
	}

	rsp, err := dao.GetCommunityList(uint(req.UserId))
	if err != nil {
		zap.S().Info("获取群列表失败", err)
		common.Error(ctx, -1, "你还没加入任何群聊")
		return
	}

	common.SuccessWithTotal(ctx, rsp, len(*rsp), "获取群列表成功")
}

// JoinGroup 加入群聊
func JoinGroup(ctx *gin.Context) {
	var req common.JoinGroupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		common.BadRequest(ctx, "参数格式错误: "+err.Error())
		return
	}

	if req.ComId == "" {
		common.Error(ctx, -1, "群名称不能为空")
		return
	}

	if req.UserId == 0 {
		common.Error(ctx, -1, "你未登录")
		return
	}

	code, err := dao.JoinCommunity(uint(req.UserId), req.ComId)
	if err != nil {
		HandleErr(code, ctx, err)
		return
	}

	common.Success(ctx, nil, "加群成功")
}

// RedisMsg 获取历史消息
func RedisMsg(c *gin.Context) {
	var req common.RedisMsgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数格式错误: "+err.Error())
		return
	}

	res := dao.GetUnreadMsg(c, req.UserId, req.UserIdB, req.Start, req.End, req.IsRev)
	common.SuccessWithTotal(c, res, len(res), "获取历史消息成功")
}
