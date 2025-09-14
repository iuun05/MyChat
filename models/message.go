package models

import (
	"github.com/fatih/set"
	"github.com/gorilla/websocket"
)

// 消息缓存相关常量
const (
	MessageCachePrefix = "msg_cache:"
	RecentMsgPrefix    = "recent_msg:"
	UnreadCountPrefix  = "unread:"
)

type Message struct {
	Model
	FormId   int64  `json:"userId"`   //信息发送者
	TargetId int64  `json:"targetId"` //信息接收者
	Type     int    //聊天类型：群聊 私聊 广播
	Media    int    //信息类型：文字 图片 音频
	Content  string //消息内容
	Pic      string `json:"url"` //图片相关
	Url      string //文件相关
	Desc     string //文件描述
	Amount   int    //其他数据大小
}

// MsgTableName 生成指定数据表名
func (m *Message) MsgTableName() string {
	return "message"
}

// Node 构造连接
type Node struct {
	Conn      *websocket.Conn //socket连接
	Addr      string          //客户端地址
	DataQueue chan []byte     //消息内容
	GroupSets set.Interface   //好友 / 群
}
