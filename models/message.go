package models

import (
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gorilla/websocket"
)

const (
	SingleMessageType = iota + 1
	CommunityMessageType
	BroadcastMessageType
	HeartBeatMessageType
	AckMessageType
)

// 消息缓存相关常量
const (
	MessageCachePrefix = "msg_cache:"
	RecentMsgPrefix    = "recent_msg:"
	UnreadCountPrefix  = "unread:"
)

type Message struct {
	Model
	FromId   int64  `json:"userId"`   //信息发送者
	TargetId int64  `json:"targetId"` //信息接收者
	Seq      int64  `json:"seq"`      //消息序号
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
	Conn            *websocket.Conn  //socket连接
	Addr            string           //客户端地址
	DataQueue       chan []byte      //消息内容
	GroupSets       set.Interface    //好友 / 群
	LastHeartbeat   time.Time        //最后心跳时间
	PendingMsgs     map[int64][]byte //待确认消息 (seq -> msg)
	LastSentSeq     int64            //最后发送的消息序号（每个Node独立）
	LastReceivedSeq int64            //最后收到的消息序号
	ExpectedSeq     int64            //期望的下一个接收序号（用于排序）
	ReceivedBuffer  map[int64][]byte //接收消息缓冲区 (seq -> msg)，用于乱序重组
	SentSeqSet      set.Interface    //已发送的序号集合（用于发送方去重）
	SeqGenerator    int64            //序号生成器（每个Node独立，避免全局竞争）
	SeqMutex        sync.Mutex       //序号生成器锁
	HeartbeatTicker *time.Ticker     //心跳定时器
	CloseChan       chan struct{}    //关闭信号
}
