package dao

import (
	"MyChat/global"
	"MyChat/models"
	"context"
	"encoding/json"
	"fmt"
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

// 心跳和消息重发相关常量
const (
	HeartbeatInterval = 30 * time.Second // 心跳发送间隔
	HeartbeatTimeout  = 90 * time.Second // 心跳超时时间
	MaxRetryCount     = 3                // 最大重试次数
	RetryInterval     = 5 * time.Second  // 重试间隔
)

// generateSeq 为Node生成唯一消息序号（每个Node独立）
func generateSeq(node *models.Node) int64 {
	node.SeqMutex.Lock()
	defer node.SeqMutex.Unlock()
	node.SeqGenerator++
	return node.SeqGenerator
}

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

	// 获取群列表
	comIds, err := GetCommunityList(uint(userId))
	if err != nil {
		zap.S().Warn("获取群列表失败，继续连接: ", err)
		comIds = &[]models.Community{}
	}

	node := &models.Node{
		Conn:            conn,
		Addr:            r.RemoteAddr,
		DataQueue:       make(chan []byte, 50),
		GroupSets:       set.New(set.ThreadSafe),
		LastHeartbeat:   time.Now(),
		PendingMsgs:     make(map[int64][]byte),
		LastSentSeq:     0,
		LastReceivedSeq: 0,
		SeqGenerator:    0,
		HeartbeatTicker: time.NewTicker(HeartbeatInterval),
		CloseChan:       make(chan struct{}),
	}

	// 添加群组到 GroupSets
	for _, com := range *comIds {
		node.GroupSets.Add(uint(com.ID))
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

		// 停止心跳定时器
		if node.HeartbeatTicker != nil {
			node.HeartbeatTicker.Stop()
		}

		// 关闭关闭信号通道
		close(node.CloseChan)

		// 清理待确认消息
		node.SeqMutex.Lock()
		node.PendingMsgs = make(map[int64][]byte)
		node.SeqMutex.Unlock()

		conn.Close()

		// 清空消息队列
		for {
			select {
			case <-node.DataQueue:
			default:
				goto done
			}
		}
	done:
		close(node.DataQueue)
		zap.S().Info("用户连接已清理: ", userId)
	}()

	done := make(chan struct{}, 3)

	//服务发送消息
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		sendProc(node, userId)
	}()

	//服务接收消息
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		recProc(node, userId)
	}()

	//心跳检测协程
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		heartbeatProc(node, userId)
	}()

	// 发送欢迎消息
	welcomeMsg := models.Message{
		FromId:   0,
		TargetId: userId,
		Type:     models.SingleMessageType,
		Content:  "欢迎进入聊天系统",
		Seq:      generateSeq(node),
	}
	welcomeData, _ := json.Marshal(welcomeMsg)
	sendMsg(userId, welcomeData)

	// 8. 等待任一协程结束
	<-done

	// 等待其他协程结束（设置超时）
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
	case <-timeout:
		zap.S().Warn("协程清理超时: ", userId)
	}

	// 确保所有协程都结束
	select {
	case <-done:
	default:
	}
}

// sendProc 发送消息处理，支持消息确认机制
func sendProc(node *models.Node, userId int64) {
	ticker := time.NewTicker(RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-node.DataQueue:
			if !ok {
				zap.S().Debug("数据队列已关闭")
				return
			}

			// 解析消息获取序号
			var msg models.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				zap.S().Warn("解析消息失败，跳过确认机制", err)
				// 如果解析失败，直接发送（可能是心跳消息等）
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入消息失败", err)
					return
				}
			}

			// 如果是心跳消息，直接发送不需要确认
			if msg.Type == models.HeartBeatMessageType {
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入心跳消息失败", err)
					return
				}
			}

			// ACK消息处理：从待确认列表中移除对应的消息
			if msg.Type == models.AckMessageType {
				if msg.TargetId == userId || msg.TargetId == 0 {
					// 从待确认列表中移除
					node.SeqMutex.Lock()
					if _, exists := node.PendingMsgs[msg.Seq]; exists {
						delete(node.PendingMsgs, msg.Seq)
						zap.S().Debugf("收到ACK确认，移除待确认消息 seq=%d", msg.Seq)
					}
					node.SeqMutex.Unlock()
				} else {
					// ACK需要路由到其他发送方Node
					rwLocker.RLock()
					targetNode, exists := clientMap[msg.TargetId]
					rwLocker.RUnlock()

					if exists {
						// 发送给发送方的DataQueue
						select {
						case targetNode.DataQueue <- data:
							zap.S().Debugf("ACK消息已路由到发送方 userId=%d", msg.TargetId)
						default:
							zap.S().Warnf("ACK消息队列已满，无法路由到 userId=%d", msg.TargetId)
						}
					} else {
						zap.S().Warnf("ACK目标用户不在线 userId=%d", msg.TargetId)
					}
				}
			}

			// 为需要确认的消息分配序号（如果还没有）
			if msg.Seq == 0 {
				msg.Seq = generateSeq(node)
				data, _ = json.Marshal(msg)
			}

			// 发送消息
			if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				zap.S().Error("写入消息失败", err)
				return
			}

			// 将消息加入待确认列表（需要确认的消息）
			if msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType {
				node.SeqMutex.Lock()
				node.PendingMsgs[msg.Seq] = data
				node.LastSentSeq = msg.Seq
				pendingCount := len(node.PendingMsgs)
				node.SeqMutex.Unlock()

				zap.S().Debugf("消息已发送，等待确认 seq=%d, 待确认消息数=%d", msg.Seq, pendingCount)
			}

		case <-ticker.C:
			// 定时检查并重发未确认消息
			retryPendingMsgs(node, userId)

		case <-node.CloseChan:
			zap.S().Debug("收到关闭信号，退出发送协程")
			return
		}
	}
}

// recProc 接收消息处理，支持心跳响应和消息确认
func recProc(node *models.Node, userId int64) {
	node.Conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout * 2))

	for {
		// 更新读取超时时间
		node.Conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout * 2))

		//获取信息
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zap.S().Error("读取消息失败", err)
			}
			return
		}

		// 解析消息
		var msg models.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			zap.S().Warn("消息解析失败", err)
			continue
		}

		// 更新最后心跳时间
		node.LastHeartbeat = time.Now()

		// 处理心跳消息
		if msg.Type == models.HeartBeatMessageType {
			// 发送心跳响应
			heartbeatAck := models.Message{
				Type:    models.HeartBeatMessageType,
				FromId:  userId,
				Content: "pong",
			}
			ackData, _ := json.Marshal(heartbeatAck)
			select {
			case node.DataQueue <- ackData:
			default:
				zap.S().Warn("心跳响应队列已满")
			}
			continue
		}

		// 处理ACK确认消息（从客户端接收的ACK）
		// ACK由接收方（当前node）发送，TargetId指向原始发送方
		// 需要将ACK路由到TargetId对应的Node，由sendProc处理
		if msg.Type == models.AckMessageType {
			// ACK应该转发到sendProc进行路由处理
			// 直接放入DataQueue，由sendProc路由到发送方
			select {
			case node.DataQueue <- data:
				zap.S().Debugf("ACK消息已放入队列等待路由 seq=%d", msg.Seq)
			default:
				zap.S().Warn("ACK确认队列已满")
			}
			continue
		}

		// 处理普通消息
		if msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType {
			// 如果消息没有序号（客户端发送的），分配序号
			if msg.Seq == 0 {
				msg.Seq = generateSeq(node)
				// 更新FromId为当前用户（客户端发送的消息）
				msg.FromId = userId
				var err error
				data, err = json.Marshal(msg)
				if err != nil {
					zap.S().Error("重新序列化消息失败", err)
					continue
				}
			}

			// 检查消息序号，防止重复处理（仅对已有序号的消息）
			if msg.Seq > 0 {
				if msg.Seq <= node.LastReceivedSeq {
					zap.S().Warnf("收到重复消息 seq=%d, 已处理序号=%d", msg.Seq, node.LastReceivedSeq)
					// 即使重复，也要发送ACK（避免发送方一直重发）
				} else {
					node.LastReceivedSeq = msg.Seq
				}

				// 发送ACK确认，TargetId指向原始发送方
				// 注意：如果消息是从客户端来的（FromId == userId），ACK不需要路由
				// 如果消息是服务器转发的（FromId != userId），ACK需要路由到FromId
				ackMsg := models.Message{
					Type:     models.AckMessageType,
					Seq:      msg.Seq,
					FromId:   userId,     // 当前用户（接收方）
					TargetId: msg.FromId, // 原始发送方
					Content:  "ack",
				}
				ackData, _ := json.Marshal(ackMsg)
				select {
				case node.DataQueue <- ackData:
				default:
					zap.S().Warn("ACK确认队列已满")
				}
			}
		}

		// 转发消息到处理逻辑
		broMsg(data)
	}
}

// heartbeatProc 心跳检测和处理协程
func heartbeatProc(node *models.Node, userId int64) {
	defer func() {
		if node.HeartbeatTicker != nil {
			node.HeartbeatTicker.Stop()
		}
	}()

	// 心跳超时检测定时器
	timeoutTicker := time.NewTicker(10 * time.Second)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-node.HeartbeatTicker.C:
			// 发送心跳
			heartbeatMsg := models.Message{
				Type:    models.HeartBeatMessageType,
				FromId:  userId,
				Content: "ping",
			}
			heartbeatData, _ := json.Marshal(heartbeatMsg)

			select {
			case node.DataQueue <- heartbeatData:
				zap.S().Debugf("发送心跳消息 userId=%d", userId)
			default:
				zap.S().Warn("心跳消息队列已满")
			}

		case <-timeoutTicker.C:
			// 检查心跳超时
			rwLocker.RLock()
			lastHeartbeat := node.LastHeartbeat
			rwLocker.RUnlock()

			if time.Since(lastHeartbeat) > HeartbeatTimeout {
				zap.S().Warnf("心跳超时，断开连接 userId=%d, 最后心跳时间=%v", userId, lastHeartbeat)
				node.Conn.Close()
				return
			}

		case <-node.CloseChan:
			zap.S().Debug("收到关闭信号，退出心跳协程")
			return
		}
	}
}

// retryPendingMsgs 重发待确认消息
func retryPendingMsgs(node *models.Node, userId int64) {
	node.SeqMutex.Lock()
	pendingCount := len(node.PendingMsgs)
	if pendingCount == 0 {
		node.SeqMutex.Unlock()
		return
	}

	// 复制待确认消息列表
	pendingCopy := make(map[int64][]byte)
	for seq, msg := range node.PendingMsgs {
		pendingCopy[seq] = msg
	}
	node.SeqMutex.Unlock()

	// 重发未确认消息
	for seq, msgData := range pendingCopy {
		zap.S().Warnf("重发未确认消息 userId=%d, seq=%d", userId, seq)

		// 直接写入连接，不经过队列避免重复
		if err := node.Conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
			zap.S().Errorf("重发消息失败 userId=%d, seq=%d, err=%v", userId, seq, err)
			// 如果重发失败，可能连接已断开，从待确认列表移除
			node.SeqMutex.Lock()
			delete(node.PendingMsgs, seq)
			node.SeqMutex.Unlock()
		}
	}
}

// UdpSendProc 完成upd数据发送, 连接到udp服务端，将全局channel中的消息体，写入udp服务端
// func UdpSendProc() {
// 	udpConn, err := net.DialUDP("udp", nil, &net.UDPAddr{
// 		//192.168.31.147
// 		IP:   net.IPv4(127, 0, 0, 1),
// 		Port: 3000,
// 		Zone: "",
// 	})
// 	if err != nil {
// 		zap.S().Info("拨号udp端口失败", err)
// 		return
// 	}

// 	defer udpConn.Close()

// 	for data := range upSendChan {
// 		_, err := udpConn.Write(data)
// 		if err != nil {
// 			zap.S().Info("写入udp消息失败", err)
// 			return
// 		}
// 		fmt.Println("数据成功发送到udp服务端:", string(data))
// 	}
// }

// UpdRecProc 完成udp数据的接收，启动udp服务，获取udp客户端的写入的消息
// func UpdRecProc() {
// 	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{
// 		IP:   net.IPv4(127, 0, 0, 1),
// 		Port: 3000,
// 	})
// 	if err != nil {
// 		zap.S().Info("监听udp端口失败", err)
// 		return
// 	}

// 	defer udpConn.Close()

// 	for {
// 		var buf [1024]byte
// 		n, err := udpConn.Read(buf[0:])
// 		if err != nil {
// 			zap.S().Info("读取udp数据失败", err)
// 			return
// 		}

// 		//处理发送逻辑
// 		dispatch(buf[0:n])
// 	}
// }

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
	case models.SingleMessageType: //私聊
		sendMsgAndSave(msg.TargetId, data)
	case models.CommunityMessageType: //群发
		sendGroupMsg(uint(msg.FromId), uint(msg.TargetId), data)
	case models.HeartBeatMessageType: // 心跳消息已在recProc中处理，这里不做处理
		zap.S().Debug("收到心跳消息")
	case models.AckMessageType: // ACK确认消息已在recProc中处理，这里不做处理
		zap.S().Debugf("收到ACK确认 seq=%d", msg.Seq)
	case models.BroadcastMessageType: //广播
		// sendBroadcastMsg(msg.TargetId, data)
		// zap.S().Warn("广播消息功能未实现")
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

// sendMsgAndSave 发送消息 并存储聊天记录到redis
func sendMsgAndSave(userId int64, msg []byte) {
	rwLocker.RLock()              //保证线程安全，上锁
	node, ok := clientMap[userId] //对方是否在线
	rwLocker.RUnlock()            //解锁

	jsonMsg := models.Message{}
	if err := json.Unmarshal(msg, &jsonMsg); err != nil {
		zap.S().Error("[sendMsgAndSave] 消息解析失败", err)
		return
	}

	// 如果消息没有序号，说明是从客户端来的，需要找到发送方的Node分配序号
	// 但这里消息已经到达sendMsgAndSave，说明已经经过了dispatch，消息应该已经有FromId
	// 为了简化，如果消息没有序号，我们就不处理确认（因为无法确定发送方Node）
	// 实际上，客户端发送的消息应该在客户端分配序号，或者服务器在recProc中分配
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

	// MySQL 持久化存储
	// 直接使用解析后的 jsonMsg 进行持久化，确保与表结构一致
	if global.DB != nil {
		if err := global.DB.Create(&jsonMsg).Error; err != nil {
			zap.S().Error("[sendMsgAndSave] Failed to persist message into MySQL ", err)
		}
	} else {
		zap.S().Warn("[sendMsgAndSave] global.DB is nil, skip MySQL persistence")
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
