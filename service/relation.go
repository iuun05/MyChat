package service

import (
	"MyChat/common"
	"MyChat/dao"
	"MyChat/models"
	"strconv"

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
	userIdStr := ctx.PostForm("userId")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		ctx.JSON(200, gin.H{
			"code":    -1,
			"message": "无效的用户ID",
		})
		return
	}

	// 获取好友列表
	friends, err := dao.FriendList(uint(userId))
	if err != nil {
		ctx.JSON(200, gin.H{
			"code":    -1,
			"message": "获取好友列表失败",
		})
		return
	}

	// 检查每个好友的在线状态
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

	common.RespOK(ctx.Writer, onlineFriends, "获取在线好友成功")
}

// RemoveFriend 移除好友
func RemoveFriend(ctx *gin.Context) {
	userIdStr := ctx.PostForm("userId")
	targetIdStr := ctx.PostForm("targetId")

	userId, err1 := strconv.Atoi(userIdStr)
	targetId, err2 := strconv.Atoi(targetIdStr)

	if err1 != nil || err2 != nil {
		ctx.JSON(200, gin.H{
			"code":    -1,
			"message": "无效的用户ID",
		})
		return
	}

	// 删除双向好友关系
	err := dao.RemoveFriend(uint(userId), uint(targetId))
	if err != nil {
		ctx.JSON(200, gin.H{
			"code":    -1,
			"message": "移除好友失败",
		})
		return
	}

	ctx.JSON(200, gin.H{
		"code":    0,
		"message": "移除好友成功",
	})
}

// GetRecentMessages 获取最近消息
func GetRecentMessages(ctx *gin.Context) {
	userIdAStr := ctx.PostForm("userIdA")
	userIdBStr := ctx.PostForm("userIdB")
	limitStr := ctx.PostForm("limit")

	userIdA, _ := strconv.ParseInt(userIdAStr, 10, 64)
	userIdB, _ := strconv.ParseInt(userIdBStr, 10, 64)
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	if limit <= 0 || limit > 100 {
		limit = 20 // 默认获取20条
	}

	messages, err := dao.GetRecentMessages(userIdA, userIdB, limit)
	if err != nil {
		ctx.JSON(200, gin.H{
			"code":    -1,
			"message": "获取消息失败",
		})
		return
	}

	common.RespOK(ctx.Writer, messages, "获取消息成功")
}

func FriendList(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Request.FormValue("userId"))
	users, err := dao.FriendList(uint(id))
	if err != nil {
		zap.S().Info("获取好友列表失败", err)
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "好友为空",
		})
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
	common.RespOKList(ctx.Writer, infos, len(infos))
}

// AddFriendByName 通过昵称加好友
func AddFriendByName(ctx *gin.Context) {
	user := ctx.PostForm("userId")
	userId, err := strconv.Atoi(user)
	if err != nil {
		zap.S().Info("类型转换失败", err)
		return
	}

	tar := ctx.PostForm("targetId")
	target, err := strconv.Atoi(tar)
	if err != nil {
		code, err := dao.AddFriendByName(uint(userId), tar)
		if err != nil {
			HandleErr(code, ctx, err)
			return
		}

	} else {
		code, err := dao.AddFriend(uint(userId), uint(target))
		if err != nil {
			HandleErr(code, ctx, err)
			return
		}
	}
	ctx.JSON(200, gin.H{
		"code":    0, //  0成功   -1失败
		"message": "添加好友成功",
	})
}

func HandleErr(code int, ctx *gin.Context, err error) {
	switch code {
	case -1:
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": err.Error(),
		})
	case 0:
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "该好友已经存在",
		})
	case -2:
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "不能添加自己",
		})

	}
}

// NewGroup 新建群聊
func NewGroup(ctx *gin.Context) {
	owner := ctx.PostForm("ownerId")
	ownerId, err := strconv.Atoi(owner)
	if err != nil {
		zap.S().Info("owner类型转换失败", err)
		return
	}

	ty := ctx.PostForm("cate")
	Type, err := strconv.Atoi(ty)
	if err != nil {
		zap.S().Info("ty类型转换失败", err)
		return
	}

	img := ctx.PostForm("icon")
	name := ctx.PostForm("name")
	desc := ctx.PostForm("desc")

	community := models.Community{}
	if ownerId == 0 {
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "您未登录",
		})
		return
	}

	if name == "" {
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "群名称不能为空",
		})
		return
	}

	if img != "" {
		community.Image = img
	}
	if desc != "" {
		community.Desc = desc
	}

	community.Name = name
	community.Type = Type
	community.OwnerId = uint(ownerId)

	code, err := dao.CreateCommunity(community)
	if err != nil {
		HandleErr(code, ctx, err)
		return
	}

	ctx.JSON(200, gin.H{
		"code":    0, //  0成功   -1失败
		"message": "建群成功",
	})
}

// GroupList 获取群列表
func GroupList(ctx *gin.Context) {
	owner := ctx.PostForm("ownerId")
	ownerId, err := strconv.Atoi(owner)
	if err != nil {
		zap.S().Info("owner类型转换失败", err)
		return
	}

	if ownerId == 0 {
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "您未登录",
		})
		return
	}

	rsp, err := dao.GetCommunityList(uint(ownerId))
	if err != nil {
		zap.S().Info("获取群列表失败", err)
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "你还没加入任何群聊",
		})
		return
	}

	common.RespOKList(ctx.Writer, rsp, len(*rsp))
}

// JoinGroup 加入群聊
func JoinGroup(ctx *gin.Context) {
	comInfo := ctx.PostForm("comId")
	if comInfo == "" {
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "群名称不能为空",
		})
		return
	}

	user := ctx.PostForm("userId")
	userId, err := strconv.Atoi(user)
	if err != nil {
		zap.S().Info("user类型转换失败")
	}
	if userId == 0 {
		ctx.JSON(200, gin.H{
			"code":    -1, //  0成功   -1失败
			"message": "你未登录",
		})
		return
	}

	code, err := dao.JoinCommunity(uint(userId), comInfo)
	if err != nil {
		HandleErr(code, ctx, err)
		return
	}

	ctx.JSON(200, gin.H{
		"code":    0, //  0成功   -1失败
		"message": "加群成功",
	})
}

func RedisMsg(c *gin.Context) {
	userIdA, _ := strconv.Atoi(c.PostForm("userIdA"))
	userIdB, _ := strconv.Atoi(c.PostForm("userIdB"))
	start, _ := strconv.Atoi(c.PostForm("start"))
	end, _ := strconv.Atoi(c.PostForm("end"))
	isRev, _ := strconv.ParseBool(c.PostForm("isRev"))
	res := dao.GetUnreadMsg(c, int64(userIdA), int64(userIdB), int64(start), int64(end), isRev)
	common.RespOKList(c.Writer, "ok", res)
}
