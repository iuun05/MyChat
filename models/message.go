package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"MyChat/global"

	"github.com/fatih/set"
	"github.com/gorilla/websocket"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
var upSendChan chan []byte = make(chan []byte, 1024)

func broMsg(data []byte) {
	upSendChan <- data
}

// rw locker
var rwLocker sync.RWMutex

// Chat    需要 ：发送者ID ，接受者ID ，消息类型，发送的内容，发送类型
func Chat(w http.ResponseWriter, r *http.Request) {
	// 1.  获取参数信息发送者userId
	query := r.URL.Query()
	Id := query.Get("userId")
	userId, err := strconv.ParseInt(Id, 10, 64)
	if err != nil {
		zap.S().Info("类型转换失败", err)
		return
	}

	// update to websocket
	//升级为socket
	var isvalida = true
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return isvalida
		},
	}).Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	node := &Node{
		Conn:      conn,
		DataQueue: make(chan []byte, 50),
		GroupSets: set.New(set.ThreadSafe),
	}

	// 将 userid 与 node 绑定
	rwLocker.Lock()
	clientMap[userId] = node
	rwLocker.Unlock()

	// defer func() {
	// 	rwLocker.Lock()
	// 	delete(clientMap, userId)
	// 	rwLocker.Unlock()
	// 	node.Conn.Close()
	// }()
	//服务发送消息
	go sendProc(node)

	//服务接收消息
	go recProc(node)

	//sendMsg(userId, []byte("欢迎进入聊天系统"))

}

func sendProc(node *Node) {
	for {
		select {
		case data := <-node.DataQueue:
			err := node.Conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				zap.S().Info("写入消息失败", err)
				return
			}
			fmt.Println("数据发送 socket 成功")
		}
	}
}

// recProc 从websocket中将消息体拿出，然后进行解析，再进行信息类型判断， 最后将消息发送至目的用户的node中
func recProc(node *Node) {
	for {
		//获取信息
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			zap.S().Info("读取消息失败", err)
			return
		}

		//这里是简单实现的一种方法
		// msg := Message{}
		// err = json.Unmarshal(data, &msg)
		// if err != nil {
		// 	zap.S().Info("json解析失败", err)
		// 	return
		// }

		// if msg.Type == 1 {
		// 	zap.S().Info("这是一条私信:", msg.Content)
		// 	rwLocker.RLock()
		// 	tarNode, ok := clientMap[msg.TargetId]
		// 	rwLocker.RUnlock()
		// 	if !ok {
		// 		zap.S().Info("不存在对应的node", msg.TargetId)
		// 		return
		// 	}

		// 	tarNode.DataQueue <- data
		// 	fmt.Println("发送成功：", string(data))
		// }
		// time.Sleep(10 * time.Millisecond)

		broMsg(data)
	}
}

func init() {
	go UdpSendProc()
	go UpdRecProc()
}

// UdpSendProc 完成upd数据发送, 连接到udp服务端，将全局channel中的消息体，写入udp服务端
func UdpSendProc() {
	udpConn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		//192.168.31.147
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 3000,
		Zone: "",
	})
	if err != nil {
		zap.S().Info("拨号udp端口失败", err)
		return
	}

	defer udpConn.Close()

	for {
		select {
		case data := <-upSendChan:
			_, err := udpConn.Write(data)
			if err != nil {
				zap.S().Info("写入udp消息失败", err)
				return
			}
			fmt.Println("数据成功发送到udp服务端:", string(data))
		}
	}
}

// UpdRecProc 完成udp数据的接收，启动udp服务，获取udp客户端的写入的消息
func UpdRecProc() {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 3000,
	})
	if err != nil {
		zap.S().Info("监听udp端口失败", err)
		return
	}

	defer udpConn.Close()

	for {
		var buf [1024]byte
		n, err := udpConn.Read(buf[0:])
		if err != nil {
			zap.S().Info("读取udp数据失败", err)
			return
		}

		//处理发送逻辑
		dispatch(buf[0:n])
	}
}

// dispatch 解析消息，聊天类型判断
func dispatch(data []byte) {
	//解析消息
	msg := Message{}
	err := json.Unmarshal(data, &msg)
	if err != nil {
		zap.S().Info("消息解析失败", err)
		return
	}

	//判断消息类型
	switch msg.Type {
	case 1: //私聊
		sendMsgAndSave(msg.TargetId, data)
	case 2: //群发
		sendGroupMsg(uint(msg.FormId), uint(msg.TargetId), data)
	}
}

// sendMs 向用户单聊发送消息
func sendMsg(id int64, msg []byte) {
	rwLocker.Lock()
	node, ok := clientMap[id]
	rwLocker.Unlock()

	if !ok {
		zap.S().Info("userID没有对应的node")
		return
	}

	zap.S().Info("targetID:", id, "node:", node)
	if ok {
		node.DataQueue <- msg
	}
}

// sendMsgTest 发送消息 并存储聊天记录到redis
func sendMsgAndSave(userId int64, msg []byte) {
	rwLocker.RLock()              //保证线程安全，上锁
	node, ok := clientMap[userId] //对方是否在线
	rwLocker.RUnlock()            //解锁

	jsonMsg := Message{}
	json.Unmarshal(msg, &jsonMsg)
	ctx := context.Background()
	targetIdStr := strconv.Itoa(int(userId))
	userIdStr := strconv.Itoa(int(jsonMsg.FormId))

	if ok {
		//如果当前用户在线，将消息转发到当前用户的websocket连接中，然后进行存储
		node.DataQueue <- msg
	}

	//userIdStr和targetIdStr进行拼接唯一key
	var key string
	if userId > jsonMsg.FormId {
		key = "msg_" + userIdStr + "_" + targetIdStr
	} else {
		key = "msg_" + targetIdStr + "_" + userIdStr
	}

	//创建记录
	res, err := global.RedisDB.ZRevRange(ctx, key, 0, -1).Result()
	if err != nil {
		fmt.Println(err)
		return
	}

	//将聊天记录写入redis缓存中
	score := float64(cap(res)) + 1
	ress, e := global.RedisDB.ZAdd(ctx, key, redis.Z{Score: score, Member: msg}).Result() //jsonMsg
	//res, e := utils.Red.Do(ctx, "zadd", key, 1, jsonMsg).Result() //备用 后续拓展 记录完整msg
	if e != nil {
		fmt.Println(e)
		return
	}
	fmt.Println(ress)
}

// sendGroupMsg 群发逻辑
func sendGroupMsg(formId, target uint, data []byte) (int, error) {
	// TODO
	userIDs, err := FindUsers(target)
	if err != nil {
		return 1, nil
	}

	for _, userId := range *userIDs {
		if formId != userId {
			sendMsgAndSave(int64(userId), data)
		}
	}

	return 0, nil
}

// RedisMsg 获取缓存里面的聊天记录
func RedisMsg(userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
	ctx := context.Background()
	userIdStr := strconv.Itoa(int(userIdA))
	targetIdStr := strconv.Itoa(int(userIdB))

	//userIdStr和targetIdStr进行拼接唯一key
	var key string
	if userIdA > userIdB {
		key = "msg_" + targetIdStr + "_" + userIdStr
	} else {
		key = "msg_" + userIdStr + "_" + targetIdStr
	}

	var rels []string
	var err error
	if isRev {
		// ZRange 默认是 从低分数到高分数，start、end 是索引（不是分数），所以第一个是最旧的消息。
		rels, err = global.RedisDB.ZRange(ctx, key, start, end).Result()
	} else {
		// ZRevRange 是 从高分数到低分数，所以第一个是最新的消息。
		rels, err = global.RedisDB.ZRevRange(ctx, key, start, end).Result()
	}
	if err != nil {
		fmt.Println(err) //没有找到
	}
	return rels
}
