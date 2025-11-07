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

// MessageDAO 消息数据访问对象
type MessageDAO struct {
	clientMap map[int64]*models.Node // 用户连接映射（WebSocket）
	rwLocker  sync.RWMutex           // 读写锁
}

// var clientMap map[int64]*models.Node = make(map[int64]*models.Node, 0)
// var rwLocker sync.RWMutex
var defaultMessageDAO *MessageDAO

// NewMessageDAO 创建MessageDAO实例
func NewMessageDAO() *MessageDAO {
	if defaultMessageDAO == nil {
		defaultMessageDAO = &MessageDAO{
			clientMap: make(map[int64]*models.Node, 0),
			rwLocker:  sync.RWMutex{},
		}
	}
	return defaultMessageDAO
}

// 心跳和消息重发相关常量
const (
	HeartbeatInterval = 30 * time.Second // 心跳发送间隔
	HeartbeatTimeout  = 90 * time.Second // 心跳超时时间
	MaxRetryCount     = 3                // 最大重试次数
	RetryInterval     = 5 * time.Second  // 重试间隔
)

// generateSeq 为Node生成唯一消息序号（每个Node独立）
func (m *MessageDAO) generateSeq(node *models.Node) int64 {
	node.SeqMutex.Lock()
	defer node.SeqMutex.Unlock()
	node.SeqGenerator++
	return node.SeqGenerator
}

// Chat 建立WebSocket连接
// 参数：
//   - userId: 连接的用户ID（发送者），通过Query参数传入
//
// 说明：
//   - 接收者ID（targetId）在客户端发送消息时，在消息的JSON中指定
//   - 消息类型、内容等也是在客户端发送消息时动态指定
func (m *MessageDAO) Chat(w http.ResponseWriter, r *http.Request) {
	// 1. 获取连接用户的ID（发送者）
	query := r.URL.Query()
	Id := query.Get("userId")
	userId, err := strconv.ParseInt(Id, 10, 64)
	if err != nil {
		zap.S().Error("WebSocket连接：userId类型转换失败", "userId", Id, zap.Error(err))
		http.Error(w, "Invalid userId", http.StatusBadRequest)
		return
	}

	zap.S().Info("WebSocket连接请求", "userId", userId, "remoteAddr", r.RemoteAddr)

	// update to websocket
	//升级为socket
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源（生产环境应该检查来源）
		},
	}).Upgrade(w, r, nil)
	if err != nil {
		zap.S().Error("WebSocket升级失败", "userId", userId, zap.Error(err))
		// Upgrade失败时，Upgrader已经处理了HTTP响应，这里只需要记录日志
		return
	}

	zap.S().Info("WebSocket连接已建立", "userId", userId)

	// 获取群列表
	comIds, err := defaultCommunityDAO.GetCommunityList(uint(userId))
	if err != nil {
		zap.S().Warn("获取群列表失败，继续连接: ", err)
		comIds = &[]models.Community{}
	}

	node := &models.Node{
		Conn:            conn,
		Addr:            r.RemoteAddr,
		DataQueue:       make(chan []byte, 256), // 增大队列容量，减少阻塞
		GroupSets:       set.New(set.ThreadSafe),
		LastHeartbeat:   time.Now(),
		PendingMsgs:     make(map[int64][]byte),
		LastSentSeq:     0,
		LastReceivedSeq: 0,
		ExpectedSeq:     1, // 期望的接收序号从1开始
		ReceivedBuffer:  make(map[int64][]byte),
		SentSeqSet:      set.New(set.ThreadSafe),
		SeqGenerator:    0,
		HeartbeatTicker: time.NewTicker(HeartbeatInterval),
		CloseChan:       make(chan struct{}),
	}

	// 启动消息顺序处理协程（用于处理乱序到达的消息）
	go m.orderedMessageProcessor(node, userId)

	// 设置写入超时，避免阻塞
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// 添加群组到 GroupSets
	for _, com := range *comIds {
		node.GroupSets.Add(uint(com.ID))
	}

	// 将 userid 与 node 绑定
	m.rwLocker.Lock()
	// clientMap[userId] = node
	if existingNode, exists := m.clientMap[userId]; exists {
		zap.S().Warn("用户重复连接，关闭旧连接: ", userId)
		existingNode.Conn.Close()
		select {
		case <-existingNode.DataQueue:
		default:
		}
		close(existingNode.DataQueue)
	}
	m.clientMap[userId] = node
	m.rwLocker.Unlock()

	// clear unread message count
	cctx := context.Background()
	if err := m.ClearUnreadCount(cctx, userId); err != nil {
		zap.S().Warn("清除未读消息计数失败: ", err)
	}

	// 5. 连接清理逻辑
	defer func() {
		m.rwLocker.Lock()
		if existingNode, exists := m.clientMap[userId]; exists && existingNode == node {
			delete(m.clientMap, userId)
		}
		m.rwLocker.Unlock()

		// 停止心跳定时器
		if node.HeartbeatTicker != nil {
			node.HeartbeatTicker.Stop()
		}

		// 关闭关闭信号通道
		close(node.CloseChan)

		// 清理待确认消息和缓冲区
		node.SeqMutex.Lock()
		node.PendingMsgs = make(map[int64][]byte)
		node.ReceivedBuffer = make(map[int64][]byte)
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
		m.sendProc(node, userId)
	}()

	//服务接收消息
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		m.recProc(node, userId)
	}()

	//心跳检测协程
	go func() {
		defer func() {
			done <- struct{}{}
		}()
		m.heartbeatProc(node, userId)
	}()

	// 发送欢迎消息（不需要确认，直接发送）
	welcomeMsg := models.Message{
		FromId:   0,
		TargetId: userId,
		Type:     models.SingleMessageType,
		Content:  "欢迎进入聊天系统",
		Seq:      0, // 欢迎消息不需要序号，不需要确认
	}
	welcomeData, _ := json.Marshal(welcomeMsg)
	// 直接发送，不通过sendMsg（避免加入待确认列表）
	node.DataQueue <- welcomeData

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
func (m *MessageDAO) sendProc(node *models.Node, userId int64) {
	ticker := time.NewTicker(RetryInterval)
	defer ticker.Stop()

	// 离线发送到 redis 中保存

	// 在线发送到 websocket 中
	// 单独的重发协程，避免阻塞主循环
	retryTicker := time.NewTicker(RetryInterval)
	defer retryTicker.Stop()
	go func() {
		for {
			select {
			case <-retryTicker.C:
				m.retryPendingMsgs(node, userId)
			case <-node.CloseChan:
				return
			}
		}
	}()

	for {
		select {
		case data, ok := <-node.DataQueue:
			if !ok {
				zap.S().Debug("数据队列已关闭")
				return
			}

			node.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

			// 解析消息获取序号
			var msg models.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				zap.S().Warn("解析消息失败，跳过确认机制", zap.Error(err))
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入消息失败", zap.Error(err))
					return
				}
				continue
			}

			// 如果是心跳消息，直接发送不需要确认
			if msg.Type == models.HeartBeatMessageType {
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入心跳消息失败", zap.Error(err))
					return
				}
				continue
			} else if msg.Type == models.AckMessageType {
				if msg.TargetId == userId || msg.TargetId == 0 {
					// 这是发给当前用户的ACK（接收方发来的确认），从待确认列表中移除
					node.SeqMutex.Lock()
					if _, exists := node.PendingMsgs[msg.Seq]; exists {
						delete(node.PendingMsgs, msg.Seq)
						zap.S().Debugf("收到ACK确认，移除待确认消息 seq=%d, userId=%d", msg.Seq, userId)
					} else {
						zap.S().Debugf("收到ACK确认，但对应的消息不在待确认列表中 seq=%d, userId=%d", msg.Seq, userId)
					}
					node.SeqMutex.Unlock()

					if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
						zap.S().Error("发送ACK给客户端失败", zap.Error(err))
					} else {
						zap.S().Debugf("ACK已发送给客户端 seq=%d, userId=%d", msg.Seq, userId)
					}
					continue // ACK处理完毕，不继续后续流程
				} else {
					// ACK需要路由到其他发送方Node（TargetId是原始发送方）
					m.rwLocker.RLock()
					targetNode, exists := m.clientMap[msg.TargetId]
					m.rwLocker.RUnlock()

					if exists {
						// 路由ACK到发送方的DataQueue，由发送方的sendProc处理
						select {
						case targetNode.DataQueue <- data:
							zap.S().Debugf("ACK消息已路由到发送方 seq=%d, from userId=%d to userId=%d", msg.Seq, userId, msg.TargetId)
						default:
							zap.S().Warnf("ACK消息队列已满，无法路由到 userId=%d", msg.TargetId)
						}
					} else {
						zap.S().Warnf("ACK目标用户不在线 userId=%d", msg.TargetId)
					}
					continue // ACK路由完毕，不继续后续流程
				}
			}

			// 为需要确认的消息分配序号（如果还没有）
			if msg.Seq == 0 {
				msg.Seq = m.generateSeq(node)
				data, _ = json.Marshal(msg)
			}

			// 发送方去重：检查是否已发送过相同序号的消息
			if (msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType) && msg.Seq > 0 && msg.FromId != 0 {
				node.SeqMutex.Lock()
				// 检查是否已发送过该序号的消息
				if node.SentSeqSet.Has(msg.Seq) {
					node.SeqMutex.Unlock()
					zap.S().Debugf("消息已发送过，跳过重复发送 seq=%d, userId=%d", msg.Seq, userId)
					continue // 跳过重复消息，不发送
				}
				// 记录已发送的序号
				node.SentSeqSet.Add(msg.Seq)
				node.SeqMutex.Unlock()
			}

			// 发送消息
			if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				zap.S().Error("写入消息失败", zap.Error(err))
				return
			}

			if (msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType) && msg.Seq > 0 && msg.FromId != 0 {
				node.SeqMutex.Lock()
				node.PendingMsgs[msg.Seq] = data
				if msg.Seq > node.LastSentSeq {
					node.LastSentSeq = msg.Seq
				}
				pendingCount := len(node.PendingMsgs)
				node.SeqMutex.Unlock()

				zap.S().Debugf("消息已发送，等待确认 seq=%d, 待确认消息数=%d", msg.Seq, pendingCount)
			}

		case <-node.CloseChan:
			zap.S().Debug("收到关闭信号，退出发送协程")
			return
		}
	}
}

// orderedMessageProcessor 顺序处理接收缓冲区的消息（处理乱序到达的消息）
func (m *MessageDAO) orderedMessageProcessor(node *models.Node, userId int64) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			node.SeqMutex.Lock()

			// 检查是否有连续的包可以处理
			processed := false
			for {
				nextData, exists := node.ReceivedBuffer[node.ExpectedSeq]
				if !exists {
					break
				}

				delete(node.ReceivedBuffer, node.ExpectedSeq)
				seq := node.ExpectedSeq
				node.ExpectedSeq++
				node.LastReceivedSeq = seq
				processed = true

				// 处理消息（消息已经在sendMsgAndSave中处理过，这里只需要更新状态）
				node.SeqMutex.Unlock()
				var msg models.Message
				if err := json.Unmarshal(nextData, &msg); err == nil {
					zap.S().Debugf("从缓冲区顺序处理消息 seq=%d, fromId=%d, userId=%d", seq, msg.FromId, userId)
				}
				node.SeqMutex.Lock()
			}

			node.SeqMutex.Unlock()

			if processed {
				zap.S().Debugf("顺序处理了缓冲区的消息，当前期望seq=%d, userId=%d", node.ExpectedSeq, userId)
			}

		case <-node.CloseChan:
			return
		}
	}
}

// recProc 接收消息处理，支持心跳响应和消息确认
func (m *MessageDAO) recProc(node *models.Node, userId int64) {
	node.Conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout * 2))

	for {
		// 更新读取超时时间
		node.Conn.SetReadDeadline(time.Now().Add(HeartbeatTimeout * 2))

		//获取信息
		_, data, err := node.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zap.S().Error("读取消息失败", zap.Error(err))
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
		} else if msg.Type == models.AckMessageType {
			select {
			case node.DataQueue <- data:
				zap.S().Debugf("ACK消息已放入队列等待路由 seq=%d", msg.Seq)
			default:
				zap.S().Warn("ACK确认队列已满")
			}
		} else if msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType {
			// 判断消息是客户端发送的（FromId == userId 或 FromId == 0），还是服务器转发的（FromId != userId）
			isClientSent := msg.FromId == userId || msg.FromId == 0

			if isClientSent {
				// 如果消息没有序号，分配序号
				if msg.Seq == 0 {
					msg.Seq = m.generateSeq(node)
					// 确保FromId设置为当前用户（发送者）
					msg.FromId = userId
					var err error
					data, err = json.Marshal(msg)
					if err != nil {
						zap.S().Error("重新序列化消息失败", zap.Error(err))
						continue
					}
				}

				m.Dispatch(data)
			} else {
				if msg.Seq > 0 {
					node.SeqMutex.Lock()
					if msg.Seq <= node.LastReceivedSeq {
						node.SeqMutex.Unlock()
						zap.S().Debugf("收到重复消息（已处理），只发送ACK seq=%d, 已处理序号=%d, userId=%d", msg.Seq, node.LastReceivedSeq, userId)
						// 重复消息：只发送ACK，不处理
						ackMsg := models.Message{
							Type:     models.AckMessageType,
							Seq:      msg.Seq,
							FromId:   userId,
							TargetId: msg.FromId,
							Content:  "ack",
						}
						ackData, _ := json.Marshal(ackMsg)
						select {
						case node.DataQueue <- ackData:
						default:
							zap.S().Warn("ACK确认队列已满")
						}
						continue
					}

					// 检查是否在缓冲区中（已收到但未处理的乱序消息）
					if _, exists := node.ReceivedBuffer[msg.Seq]; exists {
						node.SeqMutex.Unlock()
						zap.S().Debugf("消息已在缓冲区中（乱序），只发送ACK seq=%d, userId=%d", msg.Seq, userId)
						// 已在缓冲区，只发送ACK
						ackMsg := models.Message{
							Type:     models.AckMessageType,
							Seq:      msg.Seq,
							FromId:   userId,
							TargetId: msg.FromId,
							Content:  "ack",
						}
						ackData, _ := json.Marshal(ackMsg)
						select {
						case node.DataQueue <- ackData:
						default:
							zap.S().Warn("ACK确认队列已满")
						}
						continue
					}

					// 新消息：根据序号决定是直接处理还是放入缓冲区
					if msg.Seq == node.ExpectedSeq {
						// 正好是期望的序号，直接处理
						node.ExpectedSeq++
						node.LastReceivedSeq = msg.Seq

						// 处理连续的消息（从缓冲区中取出）
						msgsToProcess := [][]byte{data} // 包含当前消息
						for {
							if nextData, exists := node.ReceivedBuffer[node.ExpectedSeq]; exists {
								delete(node.ReceivedBuffer, node.ExpectedSeq)
								msgsToProcess = append(msgsToProcess, nextData)
								node.ExpectedSeq++
								node.LastReceivedSeq = node.ExpectedSeq - 1
							} else {
								break
							}
						}
						node.SeqMutex.Unlock()

						// 按顺序处理所有连续的消息（保存到Redis并放入DataQueue）
						for _, msgData := range msgsToProcess {
							var processedMsg models.Message
							if err := json.Unmarshal(msgData, &processedMsg); err == nil {
								zap.S().Debugf("顺序处理消息 seq=%d, fromId=%d, userId=%d", processedMsg.Seq, processedMsg.FromId, userId)
								// 调用sendMsgAndSave处理消息（保存并放入队列）
								m.sendMsgAndSave(userId, msgData)
							}
						}
					} else if msg.Seq > node.ExpectedSeq {
						// 乱序消息：序号大于期望序号，放入缓冲区等待（不处理，等序号到后再处理）
						node.ReceivedBuffer[msg.Seq] = data
						node.SeqMutex.Unlock()
						zap.S().Debugf("收到乱序消息，放入缓冲区 seq=%d, 期望序号=%d, userId=%d", msg.Seq, node.ExpectedSeq, userId)
					} else {
						// 序号小于期望序号（不应该发生，因为已检查过重复）
						node.SeqMutex.Unlock()
						zap.S().Warnf("收到异常序号消息 seq=%d, 期望序号=%d, userId=%d", msg.Seq, node.ExpectedSeq, userId)
					}

					// 发送ACK确认（无论是否处理）
					ackMsg := models.Message{
						Type:     models.AckMessageType,
						Seq:      msg.Seq,
						FromId:   userId,
						TargetId: msg.FromId,
						Content:  "ack",
					}
					ackData, _ := json.Marshal(ackMsg)
					select {
					case node.DataQueue <- ackData:
					default:
						zap.S().Warn("ACK确认队列已满")
					}
				} else {
					// 没有序号的消息，直接处理（不发送ACK，不进行去重排序）
					zap.S().Warnf("收到没有序号的消息 FromId=%d, TargetId=%d", msg.FromId, msg.TargetId)
					m.sendMsgAndSave(userId, data)
				}
			}
			continue
		}

		// 其他类型的消息（心跳、ACK等）不处理，已在上面处理
		zap.S().Warnf("未知消息类型或已处理: type=%d", msg.Type)
	}
}

// heartbeatProc 心跳检测和处理协程
func (m *MessageDAO) heartbeatProc(node *models.Node, userId int64) {
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
			m.rwLocker.RLock()
			lastHeartbeat := node.LastHeartbeat
			m.rwLocker.RUnlock()

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
// 注意：重发消息必须通过DataQueue，由sendProc统一写入，避免并发写入WebSocket连接
func (m *MessageDAO) retryPendingMsgs(node *models.Node, userId int64) {
	node.SeqMutex.Lock()
	pendingCount := len(node.PendingMsgs)
	if pendingCount == 0 {
		node.SeqMutex.Unlock()
		return
	}

	pendingCopy := make(map[int64][]byte)
	for seq, msg := range node.PendingMsgs {
		pendingCopy[seq] = msg
	}
	node.SeqMutex.Unlock()

	// 这样可以避免并发写入WebSocket连接（WebSocket不支持并发写入）
	for seq, msgData := range pendingCopy {

		zap.S().Debugf("重发未确认消息 userId=%d, seq=%d", userId, seq)

		// 通过DataQueue发送，由sendProc统一处理，避免并发写入
		select {
		case node.DataQueue <- msgData:
			// 成功放入队列
		case <-node.CloseChan:
			// 连接已关闭，退出重发
			return
		default:
			// 队列已满，记录警告但继续尝试其他消息
			zap.S().Warnf("重发消息队列已满，跳过重发 userId=%d, seq=%d", userId, seq)
		}
	}
}

// Dispatch 解析消息，聊天类型判断
func (m *MessageDAO) Dispatch(data []byte) {
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
		// 同步处理，保证消息顺序
		m.sendMsgAndSave(msg.TargetId, data)
	case models.CommunityMessageType: //群发
		// 同步处理群发消息，保证顺序
		m.sendGroupMsg(uint(msg.FromId), uint(msg.TargetId), data)
	case models.HeartBeatMessageType: // 心跳消息已在recProc中处理，这里不做处理
		zap.S().Debug("收到心跳消息")
	case models.AckMessageType: // ACK确认消息已在recProc中处理，这里不做处理
		zap.S().Debugf("收到ACK确认 seq=%d", msg.Seq)
	case models.BroadcastMessageType: //广播
		// sendBroadcastMsg(msg.TargetId, data)
		// zap.S().Warn("广播消息功能未实现")
	}
}

// sendMsg 向用户单聊发送消息
func (m *MessageDAO) sendMsg(id int64, msg []byte) {
	m.rwLocker.RLock()
	node, ok := m.clientMap[id]
	m.rwLocker.RUnlock()

	if !ok {
		zap.S().Info("userID没有对应的node")
		return
	}

	zap.S().Info("targetID:", id, "node:", node)
	if ok {
		node.DataQueue <- msg
	}
}

// sendMsgAndSave 发送消息并存储聊天记录
// 数据持久化策略：MySQL为主存储，Redis作为缓存
// 1. 必须保存到MySQL（持久化，即使失败也要记录）
// 2. Redis作为缓存，失败不影响消息发送，但会影响性能
// 3. 消息发送到WebSocket队列
func (m *MessageDAO) sendMsgAndSave(userId int64, msg []byte) {
	jsonMsg := models.Message{}
	if err := json.Unmarshal(msg, &jsonMsg); err != nil {
		zap.S().Error("[sendMsgAndSave] 消息解析失败", zap.Error(err))
		return
	}

	// 过滤不需要保存的消息类型（心跳、ACK等）
	if jsonMsg.Type == models.HeartBeatMessageType || jsonMsg.Type == models.AckMessageType {
		zap.S().Debugf("[sendMsgAndSave] 跳过保存系统消息 type=%d, seq=%d", jsonMsg.Type, jsonMsg.Seq)
		return
	}

	ctx := context.Background()
	targetIdStr := strconv.Itoa(int(userId))
	userIdStr := strconv.Itoa(int(jsonMsg.FromId))

	// userIdStr和targetIdStr进行拼接唯一key
	var key string
	if userId > jsonMsg.FromId {
		key = "msg_" + userIdStr + "_" + targetIdStr
	} else {
		key = "msg_" + targetIdStr + "_" + userIdStr
	}

	// 保存消息到MySQL
	if global.DB != nil {
		if err := global.DB.Create(&jsonMsg).Error; err != nil {
			zap.S().Error("[sendMsgAndSave] Failed to persist message into MySQL", zap.Error(err))
		} else {
			zap.S().Debugf("[sendMsgAndSave] 消息已保存到MySQL seq=%d, fromId=%d, toId=%d", jsonMsg.Seq, jsonMsg.FromId, userId)
		}
	} else {
		zap.S().Warn("[sendMsgAndSave] MySQL数据库连接未初始化，跳过持久化")
	}

	// 保存消息到Redis ZSet（缓存）
	count, err := global.RedisDB.ZCard(ctx, key).Result()
	if err != nil {
		zap.S().Warnf("[sendMsgAndSave] Failed to get message count from Redis: %v, 继续处理", err)
		count = 0
	}

	// 保存消息到Redis ZSet（缓存）
	score := float64(time.Now().Unix())
	memberKey := string(msg)
	if _, err = global.RedisDB.ZAdd(ctx, key, redis.Z{Score: score, Member: memberKey}).Result(); err != nil {
		zap.S().Warnf("[sendMsgAndSave] Failed to add message to Redis: %v, 消息已保存到MySQL", err)
	} else {
		zap.S().Debugf("[sendMsgAndSave] 消息已保存到Redis缓存 seq=%d", jsonMsg.Seq)

		// 清理旧消息（只保留最近1000条）
		if count > 1000 {
			if err := global.RedisDB.ZRemRangeByRank(ctx, key, 0, count-1000).Err(); err != nil {
				zap.S().Warnf("[sendMsgAndSave] Failed to clean old messages from Redis: %v", err)
			}
		}
	}

	// 更新最近消息缓存（Redis）
	recentKey := models.RecentMsgPrefix + targetIdStr
	msgInfo := map[string]any{
		"from":      jsonMsg.FromId,
		"content":   jsonMsg.Content,
		"timestamp": time.Now().Unix(),
	}
	msgData, err := json.Marshal(msgInfo)
	if err != nil {
		zap.S().Warnf("[sendMsgAndSave] Fail to Marshal msgInfo: %v", err)
	} else {
		if err := global.RedisDB.Set(ctx, recentKey, msgData, 24*time.Hour).Err(); err != nil {
			zap.S().Warnf("[sendMsgAndSave] Failed to update recent message cache: %v", err)
			// Redis缓存失败不影响主流程
		}
	}

	// 发送消息到WebSocket队列
	m.rwLocker.RLock()
	node, ok := m.clientMap[userId]
	m.rwLocker.RUnlock()

	if ok {
		// 如果当前用户在线，将消息转发到当前用户的websocket连接中
		select {
		case node.DataQueue <- msg:
			zap.S().Debugf("[sendMsgAndSave] 消息已放入WebSocket队列 userId=%d, seq=%d", userId, jsonMsg.Seq)
		default:
			zap.S().Warnf("[sendMsgAndSave] 用户消息队列已满，消息可能延迟 userId=%d, seq=%d", userId, jsonMsg.Seq)
			// 队列满时，消息已保存到MySQL和Redis，用户可以后续拉取
		}
	} else {
		// 用户不在线时，增加未读计数（Redis缓存）
		unreadKey := models.UnreadCountPrefix + targetIdStr
		if err := global.RedisDB.Incr(ctx, unreadKey).Err(); err != nil {
			zap.S().Warnf("[sendMsgAndSave] Failed to increment unread count: %v", err)
			// 未读计数失败不影响主流程，可以考虑从MySQL统计
		} else {
			global.RedisDB.Expire(ctx, unreadKey, 30*24*time.Hour)
		}
		zap.S().Debugf("[sendMsgAndSave] 用户不在线，已增加未读计数 userId=%d, seq=%d", userId, jsonMsg.Seq)
	}
}

// GetRecentMessages 获取最近消息
func (m *MessageDAO) GetRecentMessages(userIdA, userIdB int64, limit int64) ([]string, error) {
	ctx := context.Background()
	userIdStr := strconv.Itoa(int(userIdA))
	targetIdStr := strconv.Itoa(int(userIdB))

	// 确保key不受到userid顺序的影响
	var key string
	if userIdA > userIdB {
		key = "msg_" + targetIdStr + "_" + userIdStr
	} else {
		key = "msg_" + userIdStr + "_" + targetIdStr
	}

	// 获取最近消息
	messages, err := global.RedisDB.ZRevRange(ctx, key, 0, limit-1).Result()
	if err != nil {
		zap.S().Error("[GetRecentMessages] Failed to get recent messages ", err)
		return nil, err
	}
	return messages, nil
}

// ClearUnreadCount 清除未读消息计数
func (m *MessageDAO) ClearUnreadCount(ctx context.Context, userId int64) error {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	return global.RedisDB.Del(ctx, unreadKey).Err()
}

// GetUnreadCount 获取未读消息数量
func (m *MessageDAO) GetUnreadCount(ctx context.Context, userId int64) (int64, error) {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	count, err := global.RedisDB.Get(ctx, unreadKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// sendGroupMsg 群发逻辑
func (m *MessageDAO) sendGroupMsg(fromID, target uint, data []byte) (int, error) {
	userIDs, err := defaultCommunityDAO.FindUsers(target)
	if err != nil {
		return 1, nil
	}

	for _, userId := range *userIDs {
		if fromID != userId {
			m.sendMsgAndSave(int64(userId), data)
		}
	}

	return 0, nil
}

// ReadRedisMsg 获取缓存里面的聊天记录
func (m *MessageDAO) ReadRedisMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
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

// GetUnreadMsg 获取未读消息
func (m *MessageDAO) GetUnreadMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
	if msgs := m.ReadRedisMsg(ctx, userIdA, userIdB, start, end, isRev); len(msgs) > 0 {
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

// ===== 向后兼容的全局函数 =====

// generateSeq 为Node生成唯一消息序号（向后兼容）
func generateSeq(node *models.Node) int64 {
	return defaultMessageDAO.generateSeq(node)
}

// Chat 建立WebSocket连接（向后兼容）
func Chat(w http.ResponseWriter, r *http.Request) {
	defaultMessageDAO.Chat(w, r)
}

// Dispatch 解析消息，聊天类型判断（向后兼容）
func Dispatch(data []byte) {
	defaultMessageDAO.Dispatch(data)
}

// sendMsgAndSave 发送消息并存储聊天记录（向后兼容）
func sendMsgAndSave(userId int64, msg []byte) {
	defaultMessageDAO.sendMsgAndSave(userId, msg)
}

// GetRecentMessages 获取最近消息（向后兼容）
func GetRecentMessages(userIdA, userIdB int64, limit int64) ([]string, error) {
	return defaultMessageDAO.GetRecentMessages(userIdA, userIdB, limit)
}

// ClearUnreadCount 清除未读消息计数（向后兼容）
func ClearUnreadCount(ctx context.Context, userId int64) error {
	return defaultMessageDAO.ClearUnreadCount(ctx, userId)
}

// GetUnreadCount 获取未读消息数量（向后兼容）
func GetUnreadCount(ctx context.Context, userId int64) (int64, error) {
	return defaultMessageDAO.GetUnreadCount(ctx, userId)
}

// sendGroupMsg 群发逻辑（向后兼容）
func sendGroupMsg(fromID, target uint, data []byte) (int, error) {
	return defaultMessageDAO.sendGroupMsg(fromID, target, data)
}

// ReadRedisMsg 获取缓存里面的聊天记录（向后兼容）
func ReadRedisMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
	return defaultMessageDAO.ReadRedisMsg(ctx, userIdA, userIdB, start, end, isRev)
}

// GetUnreadMsg 获取未读消息（向后兼容）
func GetUnreadMsg(ctx *gin.Context, userIdA int64, userIdB int64, start int64, end int64, isRev bool) []string {
	return defaultMessageDAO.GetUnreadMsg(ctx, userIdA, userIdB, start, end, isRev)
}
