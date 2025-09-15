package dao

import (
	"MyChat/global"
	"MyChat/models"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 映射关系
var clientMap map[int64]*models.Node = make(map[int64]*models.Node, 0)
var upSendChan chan []byte = make(chan []byte, 1024)

// rw locker
var rwLocker sync.RWMutex

func broMsg(data []byte) {
	select {
	case upSendChan <- data:
	default:
		zap.S().Warn("UDP发送通道已满，消息丢弃")
	}

}

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
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}).Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	node := &models.Node{
		Conn:      conn,
		DataQueue: make(chan []byte, 50),
		GroupSets: set.New(set.ThreadSafe),
	}

	// 将 userid 与 node 绑定
	rwLocker.Lock()
	// clientMap[userId] = node
	if existingNode, exists := clientMap[userId]; exists {
		zap.S().Warn("用户重复连接，关闭旧连接: ", userId)
		existingNode.Conn.Close()
		select {
		case <-existingNode.DataQueue:
		default:
		}
		close(existingNode.DataQueue)
	}
	clientMap[userId] = node
	rwLocker.Unlock()

	// clear unread message count
	cctx := context.Background()
	if err := ClearUnreadCount(cctx, userId); err != nil {
		zap.S().Warn("清除未读消息计数失败: ", err)
	}

	// 5. 连接清理逻辑
	defer func() {
		rwLocker.Lock()
		if existingNode, exists := clientMap[userId]; exists && existingNode == node {
			delete(clientMap, userId)
		}
		rwLocker.Unlock()

		conn.Close()
		select {
		case <-node.DataQueue:
		default:
		}
		close(node.DataQueue)
		zap.S().Info("用户连接已清理: ", userId)
	}()

	done := make(chan struct{}, 2)
	//服务发送消息
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		sendProc(node)
	}()

	//服务接收消息
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		recProc(node)
	}()

	sendMsg(userId, []byte("欢迎进入聊天系统"))

	// 8. 等待任一协程结束
	<-done

	// 等待另一个协程结束（设置超时）
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
	case <-timeout:
		zap.S().Warn("协程清理超时: ", userId)
	}
}

func sendProc(node *models.Node) {
	for {
		select {
		case data, ok := <-node.DataQueue:
			if !ok {
				zap.S().Debug("数据队列已关闭")
				return
			}

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
func recProc(node *models.Node) {
	for {
		//获取信息
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			zap.S().Info("读取消息失败", err)
			return
		}

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
	msg := models.Message{}
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
		sendGroupMsg(uint(msg.FromId), uint(msg.TargetId), data)
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

	jsonMsg := models.Message{}
	json.Unmarshal(msg, &jsonMsg)
	ctx := context.Background()
	targetIdStr := strconv.Itoa(int(userId))
	userIdStr := strconv.Itoa(int(jsonMsg.FromId))

	if ok {
		//如果当前用户在线，将消息转发到当前用户的websocket连接中，然后进行存储
		node.DataQueue <- msg
	}

	//userIdStr和targetIdStr进行拼接唯一key
	// Guarantee that the key is not affected by the order of the userid
	var key string
	if userId > jsonMsg.FromId {
		key = "msg_" + userIdStr + "_" + targetIdStr
	} else {
		key = "msg_" + targetIdStr + "_" + userIdStr
	}

	// ZCARD key 是 Redis 提供的命令，用来 返回指定有序集合中的元素数量。
	count, err := global.RedisDB.ZCard(ctx, key).Result()
	if err != nil {
		zap.S().Error("[sendMsgAndSave] Failed to get number of messages ", err)
		return
	}

	// add message into ordered set
	score := float64(time.Now().Unix())
	_, err = global.RedisDB.ZAdd(ctx, key, redis.Z{Score: score, Member: msg}).Result() // redis.Z{Score: score, Member: msg}
	if err != nil {
		zap.S().Error("[sendMsgAndSave] Failed to add message to Redis ", err)
		return
	}

	if count > 1000 {
		global.RedisDB.ZRemRangeByRank(ctx, key, 0, count-1000)
	}

	// update recently cache
	recentKey := models.RecentMsgPrefix + targetIdStr
	msgInfo := map[string]any{
		"from":      jsonMsg.FromId,
		"content":   jsonMsg.Content,
		"timestamp": time.Now().Unix(),
	}
	var msgData []byte
	msgData, err = json.Marshal(msgInfo)
	if err != nil {
		zap.S().Error("[sendMsgAndSave] Fail to Marshal msgInfo ", err)
		return
	}
	global.RedisDB.Set(ctx, recentKey, msgData, 24*time.Hour)

	// Users are only counted when they are not online
	if !ok {
		unreadKey := models.UnreadCountPrefix + targetIdStr
		// unread message count + 1
		global.RedisDB.Incr(ctx, unreadKey)
		global.RedisDB.Expire(ctx, unreadKey, 30*24*time.Hour)
	}
}

func GetRecentMessages(userIdA, userIdB int64, limit int64) ([]string, error) {
	ctx := context.Background()
	userIdStr := strconv.Itoa(int(userIdA))
	targetIdStr := strconv.Itoa(int(userIdB))

	// Guarantee that the key is not affected by the order of the userid
	var key string
	if userIdA > userIdB {
		key = "msg_" + targetIdStr + "_" + userIdStr
	} else {
		key = "msg_" + userIdStr + "_" + targetIdStr
	}

	// get recently message
	messages, err := global.RedisDB.ZRevRange(ctx, key, 0, limit-1).Result()
	if err != nil {
		zap.S().Error("[GetRecentMessages] Failed to get recent messages ", err)
		return nil, err
	}
	return messages, nil
}

// ClearUnreadCount 清除未读消息计数
func ClearUnreadCount(ctx context.Context, userId int64) error {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	return global.RedisDB.Del(ctx, unreadKey).Err()
}

// GetUnreadCount 获取未读消息数量
func GetUnreadCount(ctx context.Context, userId int64) (int64, error) {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	count, err := global.RedisDB.Get(ctx, unreadKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// sendGroupMsg 群发逻辑
func sendGroupMsg(fromID, target uint, data []byte) (int, error) {
	userIDs, err := FindUsers(target)
	if err != nil {
		return 1, nil
	}

	for _, userId := range *userIDs {
		if fromID != userId {
			sendMsgAndSave(int64(userId), data)
		}
	}

	return 0, nil
}

// RedisMsg 获取缓存里面的聊天记录
func ReadRedisMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
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

func GetUnreadMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
	if msgs := ReadRedisMsg(ctx, userIdA, userIdB, start, end, isRev); len(msgs) > 0 {
		return msgs
	}

	dbmsgs := []models.Message{}
	tx := global.DB.Where(
		"(form_id = ? AND target_id = ?) OR (form_id = ? AND target_id = ?)",
		userIdA, userIdB, userIdB, userIdA,
	)

	if isRev {
		tx = tx.Order("created_at ASC")
	} else {
		tx = tx.Order("created_at DESC")
	}

	err := tx.Offset(int(start)).Limit(int(end - start + 1)).Find(&dbmsgs).Error
	if err != nil {
		fmt.Println("DB query error:", err)
		return []string{}
	}

	if len(dbmsgs) == 0 {
		return []string{}
	}

	userIdStr := strconv.Itoa(int(userIdA))
	targetIdStr := strconv.Itoa(int(userIdB))

	var key string
	if userIdA > userIdB {
		key = "msg_" + targetIdStr + "_" + userIdStr
	} else {
		key = "msg_" + userIdStr + "_" + targetIdStr
	}

	// pipe 是先收集好一堆命令，最后一次性发送给 redis 执行，减少 RTT，是一个批量写入的命令集合
	pipe := global.RedisDB.Pipeline()
	for _, m := range dbmsgs {
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(m.CreatedAt.Unix()),
			Member: m.Content,
		})
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		fmt.Printf("[GetUnreadMsg/dao] Fail to write message into redis, err %v", err.Error())
	}

	msgs := []string{}
	for _, m := range dbmsgs {
		msgs = append(msgs, m.Content)
	}

	return msgs
}
