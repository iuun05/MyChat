package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageConsumer Kafka 消息消费者
type MessageConsumer struct {
	reader         *kafka.Reader
	topic          string
	groupID        string
	messageHandler MessageHandler // 使用接口而非具体类型
	workerCount    int
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	mu             sync.RWMutex
}

// NewMessageConsumer 创建消息消费者
func NewMessageConsumer(brokers []string, topic string, groupID string, messageHandler MessageHandler, workerCount int) *MessageConsumer {
	ctx, cancel := context.WithCancel(context.Background())

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		MaxWait:        1 * time.Second,
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.LastOffset, // 从最新消息开始消费
	})

	consumer := &MessageConsumer{
		reader:         reader,
		topic:          topic,
		groupID:        groupID,
		messageHandler: messageHandler,
		workerCount:    workerCount,
		ctx:            ctx,
		cancel:         cancel,
	}

	zap.S().Infof("[Kafka Consumer] 初始化成功 brokers=%v, topic=%s, groupID=%s, workers=%d", brokers, topic, groupID, workerCount)
	return consumer
}

// Start 启动消费者（启动多个worker并发消费）
func (c *MessageConsumer) Start() {
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.consumeWorker(i)
	}

	zap.S().Infof("[Kafka Consumer] 启动 %d 个消费者worker", c.workerCount)
}

// consumeWorker 消费者worker（并发处理消息）
func (c *MessageConsumer) consumeWorker(workerID int) {
	defer c.wg.Done()

	zap.S().Infof("[Kafka Consumer] Worker %d 启动", workerID)

	for {
		select {
		case <-c.ctx.Done():
			zap.S().Infof("[Kafka Consumer] Worker %d 停止", workerID)
			return
		default:
			// 读取消息（阻塞读取）
			msg, err := c.reader.ReadMessage(c.ctx)
			if err != nil {
				if err == context.Canceled {
					zap.S().Infof("[Kafka Consumer] Worker %d 收到取消信号", workerID)
					return
				}
				zap.S().Errorf("[Kafka Consumer] Worker %d 读取消息失败", workerID, zap.Error(err))
				time.Sleep(1 * time.Second) // 错误时等待1秒后重试
				continue
			}

			// 处理消息
			if err := c.processMessage(msg); err != nil {
				zap.S().Errorf("[Kafka Consumer] Worker %d 处理消息失败", workerID, zap.Error(err))
				// 不返回，继续处理下一条消息
			}
		}
	}
}

// processMessage 处理单条消息（支持群聊和单聊）
func (c *MessageConsumer) processMessage(msg kafka.Message) error {
	// 先尝试解析为群聊消息
	var groupMsg GroupChatMessage
	if err := json.Unmarshal(msg.Value, &groupMsg); err == nil && groupMsg.GroupId > 0 {
		return c.processGroupMessage(groupMsg)
	}

	// 尝试解析为单聊消息
	var privateMsg PrivateChatMessage
	if err := json.Unmarshal(msg.Value, &privateMsg); err == nil && privateMsg.ToId > 0 {
		return c.processPrivateMessage(privateMsg)
	}

	// 两种消息类型都不匹配，记录错误
	zap.S().Errorf("[processMessage] 未知消息类型 offset=%d, topic=%s", msg.Offset, c.topic)
	return fmt.Errorf("未知消息类型，无法解析")
}

// processGroupMessage 处理群聊消息
func (c *MessageConsumer) processGroupMessage(groupMsg GroupChatMessage) error {
	zap.S().Debugf("[processGroupMessage] 收到群消息 groupId=%d, seq=%d", groupMsg.GroupId, groupMsg.Seq)

	// 获取群成员列表
	memberInterfaces, err := c.messageHandler.GetShardDAO().GetGroupMembers(groupMsg.GroupId)
	if err != nil {
		zap.S().Errorf("[processGroupMessage] 获取群成员失败 groupId=%d", groupMsg.GroupId, zap.Error(err))
		return fmt.Errorf("获取群成员失败: %w", err)
	}

	// 并发推送给所有成员（不保存，只推送）
	var wg sync.WaitGroup
	pushCount := 0

	for _, memberInterface := range memberInterfaces {
		// 类型断言
		member, ok := memberInterface.(GroupMember)
		if !ok {
			zap.S().Warnf("[processGroupMessage] 成员类型断言失败 groupId=%d", groupMsg.GroupId)
			continue
		}

		// 跳过发送者自己
		if int64(member.GetUserId()) == groupMsg.FromId {
			continue
		}

		// 只推送给正常状态的成员
		if member.GetStatus() != 1 { // 1 = GroupMemberStatusNormal
			continue
		}

		wg.Add(1)
		pushCount++

		go func(userId int64) {
			defer wg.Done()
			// 只推送，不保存（群消息已统一保存）
			c.messageHandler.PushMessageToUser(userId, groupMsg.Message)
		}(int64(member.GetUserId()))
	}

	// 等待所有推送完成
	wg.Wait()

	zap.S().Debugf("[processGroupMessage] 群消息已处理 groupId=%d, seq=%d, members=%d, pushed=%d",
		groupMsg.GroupId, groupMsg.Seq, len(memberInterfaces), pushCount)

	return nil
}

// processPrivateMessage 处理单聊消息
func (c *MessageConsumer) processPrivateMessage(privateMsg PrivateChatMessage) error {
	zap.S().Debugf("[processPrivateMessage] 收到单聊消息 fromId=%d, toId=%d, seq=%d", privateMsg.FromId, privateMsg.ToId, privateMsg.Seq)

	// 单聊消息直接推送给接收者（消息已经在发送前持久化到数据库）
	c.messageHandler.PushMessageToUser(privateMsg.ToId, privateMsg.Message)

	zap.S().Debugf("[processPrivateMessage] 单聊消息已处理 fromId=%d, toId=%d, seq=%d",
		privateMsg.FromId, privateMsg.ToId, privateMsg.Seq)

	return nil
}

// Stop 停止消费者
func (c *MessageConsumer) Stop() error {
	zap.S().Info("[Kafka Consumer] 正在停止消费者...")

	// 取消context，通知所有worker停止
	c.cancel()

	// 等待所有worker完成
	c.wg.Wait()

	// 关闭reader
	if err := c.reader.Close(); err != nil {
		zap.S().Errorf("[Kafka Consumer] 关闭reader失败", zap.Error(err))
		return err
	}

	zap.S().Info("[Kafka Consumer] 消费者已停止")
	return nil
}

// GetStats 获取消费者统计信息
func (c *MessageConsumer) GetStats() kafka.ReaderStats {
	return c.reader.Stats()
}
