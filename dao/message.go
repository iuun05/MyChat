package dao

import (
	"MyChat/global"
	"MyChat/models"
	"context"
	"encoding/binary"
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

// UDP可靠传输相关结构
type UDPPacket struct {
	Type    uint8  // 包类型：0=数据, 1=ACK, 2=重传请求
	Seq     uint32 // 序号（发送方分配）
	AckSeq  uint32 // ACK序号（确认已收到的最大序号）
	DataLen uint16 // 数据长度
	Data    []byte // 数据内容
}

// UDP发送状态
type UDPSender struct {
	NextSeq        uint32                // 下一个要发送的序号
	PendingPackets map[uint32]*UDPPacket // 待确认的数据包
	PendingMutex   sync.RWMutex          // 保护待确认列表
	RetryTicker    *time.Ticker          // 重传定时器
}

// UDP接收状态
type UDPReceiver struct {
	ExpectedSeq uint32                // 期望的下一个序号
	Buffer      map[uint32]*UDPPacket // 乱序缓冲区
	BufferMutex sync.RWMutex          // 保护缓冲区
	LastAckSeq  uint32                // 最后确认的序号
}

var (
	udpSender     *UDPSender
	udpReceiver   *UDPReceiver
	udpStateMutex sync.RWMutex
)

// UDP包类型常量
const (
	UDPPacketTypeData       = 0 // 数据包
	UDPPacketTypeAck        = 1 // ACK确认包
	UDPPacketTypeRetransmit = 2 // 重传请求包
)

// 心跳和消息重发相关常量
const (
	HeartbeatInterval = 30 * time.Second // 心跳发送间隔
	HeartbeatTimeout  = 90 * time.Second // 心跳超时时间
	MaxRetryCount     = 3                // 最大重试次数
	RetryInterval     = 5 * time.Second  // 重试间隔
	UDPRetryInterval  = 2 * time.Second  // UDP重传间隔
	UDPMaxRetryCount  = 5                // UDP最大重试次数
	UDPBufferSize     = 1000             // UDP接收缓冲区大小
	UDPPacketTimeout  = 10 * time.Second // UDP包超时时间
)

// UDP配置
var (
	UDPListenAddr = &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 3000,
	}
	UDPRemoteAddr = &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 3001, // 远程UDP服务器端口（如果有）
	}
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

// encodeUDPPacket 编码UDP包为字节流
func encodeUDPPacket(pkt *UDPPacket) ([]byte, error) {
	// 包格式：Type(1) + Seq(4) + AckSeq(4) + DataLen(2) + Data(N)
	dataLen := uint16(len(pkt.Data))
	buf := make([]byte, 1+4+4+2+dataLen)

	offset := 0
	buf[offset] = pkt.Type
	offset++

	binary.BigEndian.PutUint32(buf[offset:], pkt.Seq)
	offset += 4

	binary.BigEndian.PutUint32(buf[offset:], pkt.AckSeq)
	offset += 4

	binary.BigEndian.PutUint16(buf[offset:], dataLen)
	offset += 2

	copy(buf[offset:], pkt.Data)
	return buf, nil
}

// decodeUDPPacket 从字节流解码UDP包
func decodeUDPPacket(data []byte) (*UDPPacket, error) {
	if len(data) < 11 { // 最小包大小：1+4+4+2
		return nil, fmt.Errorf("UDP包太小")
	}

	pkt := &UDPPacket{}
	offset := 0

	pkt.Type = data[offset]
	offset++

	pkt.Seq = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	pkt.AckSeq = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	pkt.DataLen = binary.BigEndian.Uint16(data[offset:])
	offset += 2

	if len(data) < offset+int(pkt.DataLen) {
		return nil, fmt.Errorf("UDP包数据不完整")
	}

	pkt.Data = make([]byte, pkt.DataLen)
	copy(pkt.Data, data[offset:offset+int(pkt.DataLen)])

	return pkt, nil
}

// initUDPSender 初始化UDP发送器
func initUDPSender() {
	udpStateMutex.Lock()
	defer udpStateMutex.Unlock()

	if udpSender == nil {
		udpSender = &UDPSender{
			NextSeq:        1,
			PendingPackets: make(map[uint32]*UDPPacket),
			RetryTicker:    time.NewTicker(UDPRetryInterval),
		}
	}
}

// initUDPReceiver 初始化UDP接收器
func initUDPReceiver() {
	udpStateMutex.Lock()
	defer udpStateMutex.Unlock()

	if udpReceiver == nil {
		udpReceiver = &UDPReceiver{
			ExpectedSeq: 1,
			Buffer:      make(map[uint32]*UDPPacket),
			LastAckSeq:  0,
		}
	}
}

// Chat 建立WebSocket连接
// 参数：
//   - userId: 连接的用户ID（发送者），通过Query参数传入
//
// 说明：
//   - 接收者ID（targetId）在客户端发送消息时，在消息的JSON中指定
//   - 消息类型、内容等也是在客户端发送消息时动态指定
func Chat(w http.ResponseWriter, r *http.Request) {
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
				zap.S().Warn("解析消息失败，跳过确认机制", zap.Error(err))
				// 如果解析失败，直接发送（可能是心跳消息等）
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入消息失败", zap.Error(err))
					return
				}
			}

			// 如果是心跳消息，直接发送不需要确认
			if msg.Type == models.HeartBeatMessageType {
				if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					zap.S().Error("写入心跳消息失败", zap.Error(err))
					return
				}
			}

			// ACK消息处理：从待确认列表中移除对应的消息（ACK是服务器内部消息，不发送给客户端）
			if msg.Type == models.AckMessageType {
				if msg.TargetId == userId || msg.TargetId == 0 {
					// 这是发给当前用户的ACK，从待确认列表中移除
					node.SeqMutex.Lock()
					if _, exists := node.PendingMsgs[msg.Seq]; exists {
						delete(node.PendingMsgs, msg.Seq)
						zap.S().Debugf("收到ACK确认，移除待确认消息 seq=%d", msg.Seq)
					} else {
						zap.S().Debugf("收到ACK确认，但对应的消息不在待确认列表中 seq=%d", msg.Seq)
					}
					node.SeqMutex.Unlock()
				} else {
					// ACK需要路由到其他发送方Node（TargetId是原始发送方）
					rwLocker.RLock()
					targetNode, exists := clientMap[msg.TargetId]
					rwLocker.RUnlock()

					if exists {
						// 发送给发送方的DataQueue，由发送方的sendProc处理
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
				// ACK是服务器内部消息，不发送给WebSocket客户端
				continue
			}

			// 为需要确认的消息分配序号（如果还没有）
			if msg.Seq == 0 {
				msg.Seq = generateSeq(node)
				data, _ = json.Marshal(msg)
			}

			// 发送消息
			if err := node.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				zap.S().Error("写入消息失败", zap.Error(err))
				return
			}

			// 将消息加入待确认列表（需要确认的消息，且序号不为0）
			// 注意：FromId=0的消息（如欢迎消息）不需要确认
			if (msg.Type == models.SingleMessageType || msg.Type == models.CommunityMessageType) && msg.Seq > 0 && msg.FromId != 0 {
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
			// 判断消息是客户端发送的（FromId == userId 或 FromId == 0），还是服务器转发的（FromId != userId）
			isClientSent := msg.FromId == userId || msg.FromId == 0

			if isClientSent {
				// 这是客户端发送的消息，需要路由给目标用户
				// 如果消息没有序号，分配序号
				if msg.Seq == 0 {
					msg.Seq = generateSeq(node)
					// 确保FromId设置为当前用户（发送者）
					msg.FromId = userId
					var err error
					data, err = json.Marshal(msg)
					if err != nil {
						zap.S().Error("重新序列化消息失败", zap.Error(err))
						continue
					}
				}
				// 客户端发送的消息，直接转发给目标用户，不需要ACK和序号检查
				Dispatch(data)
			} else {
				// 这是服务器转发的消息（从其他用户接收到的）
				// 检查消息序号，防止重复处理（仅对已有序号的消息）
				if msg.Seq > 0 {
					if msg.Seq <= node.LastReceivedSeq {
						zap.S().Warnf("收到重复消息 seq=%d, 已处理序号=%d", msg.Seq, node.LastReceivedSeq)
						// 即使重复，也要发送ACK（避免发送方一直重发）
					} else {
						node.LastReceivedSeq = msg.Seq
					}

					// 发送ACK确认，TargetId指向原始发送方
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
				// 服务器转发的消息，直接显示给当前用户（已经在sendMsgAndSave中放入DataQueue）
				// 不需要再次Dispatch，避免重复处理
			}
			continue
		}

		// 其他类型的消息（心跳、ACK等）不处理，已在上面处理
		zap.S().Warnf("未知消息类型或已处理: type=%d", msg.Type)
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
		// 重发是正常机制，使用Debug级别
		zap.S().Debugf("重发未确认消息 userId=%d, seq=%d", userId, seq)

		// 直接写入连接，不经过队列避免重复
		if err := node.Conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
			zap.S().Errorf("重发消息失败 userId=%d, seq=%d", userId, seq, zap.Error(err))
			// 如果重发失败，可能连接已断开，从待确认列表移除
			node.SeqMutex.Lock()
			delete(node.PendingMsgs, seq)
			node.SeqMutex.Unlock()
		}
	}
}

// UdpSendProc 完成UDP数据发送，支持可靠传输
func UdpSendProc() {
	initUDPSender()

	udpConn, err := net.DialUDP("udp", nil, UDPListenAddr)
	if err != nil {
		zap.S().Error("UDP拨号失败", zap.Error(err))
		return
	}
	defer udpConn.Close()

	// 启动重传协程
	go udpRetransmitWorker(udpConn)

	// 发送数据协程
	for data := range upSendChan {
		// 创建UDP包
		var lastAckSeq uint32
		udpStateMutex.RLock()
		if udpReceiver != nil {
			udpReceiver.BufferMutex.RLock()
			lastAckSeq = udpReceiver.LastAckSeq
			udpReceiver.BufferMutex.RUnlock()
		}
		udpStateMutex.RUnlock()

		pkt := &UDPPacket{
			Type:    UDPPacketTypeData,
			Seq:     udpSender.NextSeq,
			AckSeq:  lastAckSeq, // 携带接收方最后确认的序号（用于双向确认）
			DataLen: uint16(len(data)),
			Data:    data,
		}

		// 编码包
		packetData, err := encodeUDPPacket(pkt)
		if err != nil {
			zap.S().Error("编码UDP包失败", err)
			continue
		}

		// 发送UDP包
		_, err = udpConn.Write(packetData)
		if err != nil {
			zap.S().Error("发送UDP包失败", err)
			continue
		}

		// 添加到待确认列表
		udpSender.PendingMutex.Lock()
		udpSender.PendingPackets[pkt.Seq] = pkt
		udpSender.NextSeq++
		udpSender.PendingMutex.Unlock()

		zap.S().Debugf("UDP数据包已发送 seq=%d, dataLen=%d", pkt.Seq, len(data))
	}
}

// udpRetransmitWorker UDP重传工作协程
func udpRetransmitWorker(conn *net.UDPConn) {
	ticker := time.NewTicker(UDPRetryInterval)
	defer ticker.Stop()

	for range ticker.C {
		udpSender.PendingMutex.RLock()
		if len(udpSender.PendingPackets) == 0 {
			udpSender.PendingMutex.RUnlock()
			continue
		}

		// 复制待重传的包列表
		packetsToRetry := make([]*UDPPacket, 0)
		for _, pkt := range udpSender.PendingPackets {
			packetsToRetry = append(packetsToRetry, pkt)
		}
		udpSender.PendingMutex.RUnlock()

		// 重传未确认的包
		for _, pkt := range packetsToRetry {
			packetData, err := encodeUDPPacket(pkt)
			if err != nil {
				zap.S().Errorf("重传编码UDP包失败 seq=%d", pkt.Seq, err)
				continue
			}

			_, err = conn.Write(packetData)
			if err != nil {
				zap.S().Errorf("重传UDP包失败 seq=%d", pkt.Seq, zap.Error(err))
			} else {
				// UDP重传是正常机制，使用Debug级别
				zap.S().Debugf("重传UDP包 seq=%d", pkt.Seq)
			}
		}
	}
}

// UpdRecProc 完成UDP数据的接收，支持可靠传输和顺序重组
func UpdRecProc() {
	initUDPReceiver()

	udpConn, err := net.ListenUDP("udp", UDPListenAddr)
	if err != nil {
		zap.S().Error("监听UDP端口失败", err)
		return
	}
	defer udpConn.Close()

	zap.S().Info("UDP接收服务已启动", UDPListenAddr)

	// 启动顺序处理协程
	go udpOrderedProcessor()

	for {
		buf := make([]byte, 1500) // UDP最大包大小
		n, addr, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			zap.S().Error("读取UDP数据失败", err)
			continue
		}

		// 解码UDP包
		pkt, err := decodeUDPPacket(buf[:n])
		if err != nil {
			zap.S().Warn("解码UDP包失败", err, "from", addr)
			continue
		}

		// 处理不同类型的包
		switch pkt.Type {
		case UDPPacketTypeData:
			handleUDPDataPacket(pkt, udpConn, addr)
		case UDPPacketTypeAck:
			handleUDPAckPacket(pkt)
		case UDPPacketTypeRetransmit:
			handleUDPRetransmitRequest(pkt, udpConn, addr)
		default:
			zap.S().Warnf("未知UDP包类型: %d", pkt.Type)
		}
	}
}

// handleUDPDataPacket 处理UDP数据包
func handleUDPDataPacket(pkt *UDPPacket, conn *net.UDPConn, addr *net.UDPAddr) {
	udpReceiver.BufferMutex.Lock()
	defer udpReceiver.BufferMutex.Unlock()

	seq := pkt.Seq

	// 如果序号小于期望序号，说明是重复包，仍然发送ACK
	if seq < udpReceiver.ExpectedSeq {
		zap.S().Debugf("收到重复UDP包 seq=%d, 期望=%d", seq, udpReceiver.ExpectedSeq)
		sendUDPAck(conn, addr, udpReceiver.LastAckSeq)
		return
	}

	// 如果序号等于期望序号，直接处理
	if seq == udpReceiver.ExpectedSeq {
		udpReceiver.ExpectedSeq++
		udpReceiver.LastAckSeq = seq

		// 处理消息
		Dispatch(pkt.Data)

		// 检查缓冲区中是否有连续的包可以处理
		for {
			nextPkt, exists := udpReceiver.Buffer[udpReceiver.ExpectedSeq]
			if !exists {
				break
			}
			udpReceiver.ExpectedSeq++
			udpReceiver.LastAckSeq = nextPkt.Seq
			delete(udpReceiver.Buffer, nextPkt.Seq)
			Dispatch(nextPkt.Data)
		}

		// 发送ACK
		sendUDPAck(conn, addr, udpReceiver.LastAckSeq)
	} else {
		// 序号大于期望序号，放入缓冲区（乱序）
		if len(udpReceiver.Buffer) < UDPBufferSize {
			udpReceiver.Buffer[seq] = pkt
			zap.S().Debugf("UDP包乱序，放入缓冲区 seq=%d, 期望=%d", seq, udpReceiver.ExpectedSeq)
		} else {
			zap.S().Warnf("UDP接收缓冲区已满，丢弃包 seq=%d", seq)
		}
		// 仍然发送ACK（确认收到但未处理）
		sendUDPAck(conn, addr, udpReceiver.LastAckSeq)
	}
}

// handleUDPAckPacket 处理UDP ACK包
func handleUDPAckPacket(pkt *UDPPacket) {
	udpSender.PendingMutex.Lock()
	defer udpSender.PendingMutex.Unlock()

	ackSeq := pkt.AckSeq

	// 移除已确认的包
	packetsToRemove := make([]uint32, 0)
	for seq := range udpSender.PendingPackets {
		if seq <= ackSeq {
			packetsToRemove = append(packetsToRemove, seq)
		}
	}

	for _, seq := range packetsToRemove {
		delete(udpSender.PendingPackets, seq)
	}

	if len(packetsToRemove) > 0 {
		zap.S().Debugf("收到UDP ACK，移除 %d 个已确认包，最大seq=%d", len(packetsToRemove), ackSeq)
	}
}

// handleUDPRetransmitRequest 处理重传请求
func handleUDPRetransmitRequest(pkt *UDPPacket, conn *net.UDPConn, addr *net.UDPAddr) {
	// 接收方请求重传某个序号的数据包
	// 这里简化处理：如果还在缓冲区中，直接发送ACK即可（因为可能已经处理过了）
	zap.S().Debugf("收到重传请求 seq=%d", pkt.Seq)
	sendUDPAck(conn, addr, udpReceiver.LastAckSeq)
}

// sendUDPAck 发送UDP ACK包
func sendUDPAck(conn *net.UDPConn, addr *net.UDPAddr, ackSeq uint32) {
	pkt := &UDPPacket{
		Type:    UDPPacketTypeAck,
		Seq:     0, // ACK包不需要序号
		AckSeq:  ackSeq,
		DataLen: 0,
		Data:    nil,
	}

	packetData, err := encodeUDPPacket(pkt)
	if err != nil {
		zap.S().Error("编码UDP ACK包失败", err)
		return
	}

	_, err = conn.WriteToUDP(packetData, addr)
	if err != nil {
		zap.S().Error("发送UDP ACK失败", err)
	} else {
		zap.S().Debugf("发送UDP ACK ackSeq=%d", ackSeq)
	}
}

// udpOrderedProcessor 顺序处理缓冲区中的数据包
func udpOrderedProcessor() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		udpReceiver.BufferMutex.Lock()

		// 检查是否有连续的包可以处理
		processed := false
		for {
			nextPkt, exists := udpReceiver.Buffer[udpReceiver.ExpectedSeq]
			if !exists {
				break
			}
			udpReceiver.ExpectedSeq++
			udpReceiver.LastAckSeq = nextPkt.Seq
			delete(udpReceiver.Buffer, nextPkt.Seq)
			Dispatch(nextPkt.Data)
			processed = true
		}

		udpReceiver.BufferMutex.Unlock()

		if processed {
			zap.S().Debugf("顺序处理了缓冲区的UDP包，当前期望seq=%d", udpReceiver.ExpectedSeq)
		}
	}
}

// dispatch 解析消息，聊天类型判断
func Dispatch(data []byte) {
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
	ctx := context.Background()
	targetIdStr := strconv.Itoa(int(userId))
	userIdStr := strconv.Itoa(int(jsonMsg.FromId))

	if ok {
		//如果当前用户在线，将消息转发到当前用户的websocket连接中，然后进行存储
		node.DataQueue <- msg
	}

	//userIdStr和targetIdStr进行拼接唯一key
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

	score := float64(time.Now().Unix())
	_, err = global.RedisDB.ZAdd(ctx, key, redis.Z{Score: score, Member: msg}).Result() // redis.Z{Score: score, Member: msg}
	if err != nil {
		zap.S().Error("[sendMsgAndSave] Failed to add message to Redis ", err)
		return
	}

	if count > 1000 {
		global.RedisDB.ZRemRangeByRank(ctx, key, 0, count-1000)
	}

	// 更新最近消息缓存
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

	// 用户不在线时，计数+1
	if !ok {
		unreadKey := models.UnreadCountPrefix + targetIdStr
		// 未读消息计数+1
		global.RedisDB.Incr(ctx, unreadKey)
		global.RedisDB.Expire(ctx, unreadKey, 30*24*time.Hour)
	}

	// MySQL 持久化存储
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

// 清除未读消息计数
func ClearUnreadCount(ctx context.Context, userId int64) error {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	return global.RedisDB.Del(ctx, unreadKey).Err()
}

// 获取未读消息数量
func GetUnreadCount(ctx context.Context, userId int64) (int64, error) {
	unreadKey := models.UnreadCountPrefix + strconv.Itoa(int(userId))
	count, err := global.RedisDB.Get(ctx, unreadKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// 群发逻辑
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

// 获取缓存里面的聊天记录
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
