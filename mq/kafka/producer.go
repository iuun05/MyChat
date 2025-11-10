package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

var (
	defaultMessageProducer *MessageProducer
	producerOnce           sync.Once
)

// GroupChatMessage Kafka 群聊消息结构
type GroupChatMessage struct {
	GroupId   int64  `json:"groupId"`   // 群ID
	FromId    int64  `json:"fromId"`    // 发送者ID
	Message   []byte `json:"message"`   // 消息内容（JSON格式）
	Seq       int64  `json:"seq"`       // 消息序号
	MessageId string `json:"messageId"` // 消息ID
}

// MessageProducer Kafka 消息生产者
type MessageProducer struct {
	writer  *kafka.Writer
	topic   string
	brokers []string
	ctx     context.Context
	mu      sync.RWMutex
}

// NewMessageProducer 创建消息生产者（单例模式）
func NewMessageProducer(brokers []string, topic string) *MessageProducer {
	producerOnce.Do(func() {
		writer := &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{}, // 负载均衡策略
			Async:        false,               // 同步写入，保证消息顺序
			RequiredAcks: kafka.RequireAll,    // 等待所有副本确认
		}

		defaultMessageProducer = &MessageProducer{
			writer:  writer,
			topic:   topic,
			brokers: brokers,
			ctx:     context.Background(),
		}

		zap.S().Infof("[Kafka Producer] 初始化成功 brokers=%v, topic=%s", brokers, topic)
	})

	return defaultMessageProducer
}

// SendGroupChatMessage 发送群聊消息到 Kafka
func (p *MessageProducer) SendGroupChatMessage(groupId int64, fromId int64, messageData []byte, seq int64, messageId string) error {
	// 构造 Kafka 消息
	groupMsg := GroupChatMessage{
		GroupId:   groupId,
		FromId:    fromId,
		Message:   messageData,
		Seq:       seq,
		MessageId: messageId,
	}

	// 序列化为 JSON
	msgBytes, err := json.Marshal(groupMsg)
	if err != nil {
		zap.S().Errorf("[SendGroupChatMessage] 序列化消息失败 groupId=%d", groupId, zap.Error(err))
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 使用群ID作为分区键，确保同一群的消息有序
	partitionKey := fmt.Sprintf("group_%d", groupId)

	// 发送到 Kafka
	msg := kafka.Message{
		Key:   []byte(partitionKey),
		Value: msgBytes,
		Headers: []kafka.Header{
			{Key: "groupId", Value: []byte(fmt.Sprintf("%d", groupId))},
			{Key: "fromId", Value: []byte(fmt.Sprintf("%d", fromId))},
			{Key: "seq", Value: []byte(fmt.Sprintf("%d", seq))},
		},
	}

	if err := p.writer.WriteMessages(p.ctx, msg); err != nil {
		zap.S().Errorf("[SendGroupChatMessage] 发送消息到Kafka失败 groupId=%d, seq=%d", groupId, seq, zap.Error(err))
		return fmt.Errorf("发送消息到Kafka失败: %w", err)
	}

	zap.S().Debugf("[SendGroupChatMessage] 群消息已发送到Kafka groupId=%d, seq=%d, messageId=%s", groupId, seq, messageId)
	return nil
}

// Close 关闭生产者
func (p *MessageProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// GetDefaultProducer 获取默认生产者
func GetDefaultProducer() *MessageProducer {
	return defaultMessageProducer
}
