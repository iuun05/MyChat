package models

import (
	"net/http"
	"sync"

	"github.com/fatih/set"
	"golang.org/x/net/websocket"
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

// 映射关系
var clientMap map[int64]*Node = make(map[int64]*Node, 0)

// rw locker
var rwLocker sync.RWMutex

// Chat    需要 ：发送者ID ，接受者ID ，消息类型，发送的内容，发送类型
func Chat(w http.ResponseWriter, r *http.Request) {

}
