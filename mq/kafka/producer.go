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

// PrivateChatMessage Kafka 单聊消息结构
type PrivateChatMessage struct {
	FromId    int64  `json:"fromId"`    // 发送者ID
	ToId      int64  `json:"toId"`      // 接收者ID
	Message   []byte `json:"message"`   // 消息内容（JSON格式）
	Seq       int64  `json:"seq"`       // 消息序号
	MessageId string `json:"messageId"` // 消息ID
}

// MessageProducer Kafka 消息生产者
type MessageProducer struct {
	groupWriter   *kafka.Writer // 群聊消息writer
	privateWriter *kafka.Writer // 单聊消息writer
	groupTopic    string        // 群聊topic
	privateTopic  string        // 单聊topic
	brokers       []string
	ctx           context.Context
	mu            sync.RWMutex
}

// NewMessageProducer 创建消息生产者（单例模式）
// 支持群聊和单聊两个topic
func NewMessageProducer(brokers []string, groupTopic string, privateTopic string) *MessageProducer {
	producerOnce.Do(func() {
		// 创建群聊消息writer
		groupWriter := &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        groupTopic,
			Balancer:     &kafka.LeastBytes{}, // 负载均衡策略
			Async:        false,               // 同步写入，保证消息顺序
			RequiredAcks: kafka.RequireAll,    // 等待所有副本确认
		}

		// 创建单聊消息writer
		privateWriter := &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        privateTopic,
			Balancer:     &kafka.LeastBytes{}, // 负载均衡策略
			Async:        false,               // 同步写入，保证消息顺序
			RequiredAcks: kafka.RequireAll,    // 等待所有副本确认
		}

		defaultMessageProducer = &MessageProducer{
			groupWriter:   groupWriter,
			privateWriter: privateWriter,
			groupTopic:    groupTopic,
			privateTopic:  privateTopic,
			brokers:       brokers,
			ctx:           context.Background(),
		}

		zap.S().Infof("[Kafka Producer] 初始化成功 brokers=%v, groupTopic=%s, privateTopic=%s", brokers, groupTopic, privateTopic)
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

	if err := p.groupWriter.WriteMessages(p.ctx, msg); err != nil {
		zap.S().Errorf("[SendGroupChatMessage] 发送消息到Kafka失败 groupId=%d, seq=%d", groupId, seq, zap.Error(err))
		return fmt.Errorf("发送消息到Kafka失败: %w", err)
	}

	zap.S().Debugf("[SendGroupChatMessage] 群消息已发送到Kafka groupId=%d, seq=%d, messageId=%s", groupId, seq, messageId)
	return nil
}

// SendPrivateChatMessage 发送单聊消息到 Kafka
func (p *MessageProducer) SendPrivateChatMessage(fromId int64, toId int64, messageData []byte, seq int64, messageId string) error {
	// 构造 Kafka 消息
	privateMsg := PrivateChatMessage{
		FromId:    fromId,
		ToId:      toId,
		Message:   messageData,
		Seq:       seq,
		MessageId: messageId,
	}

	// 序列化为 JSON
	msgBytes, err := json.Marshal(privateMsg)
	if err != nil {
		zap.S().Errorf("[SendPrivateChatMessage] 序列化消息失败 fromId=%d, toId=%d", fromId, toId, zap.Error(err))
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 使用接收者ID作为分区键，确保同一接收者的消息有序
	// 为了更好的负载均衡，可以使用 fromId_toId 作为key
	partitionKey := fmt.Sprintf("private_%d_%d", fromId, toId)

	// 发送到 Kafka
	msg := kafka.Message{
		Key:   []byte(partitionKey),
		Value: msgBytes,
		Headers: []kafka.Header{
			{Key: "fromId", Value: []byte(fmt.Sprintf("%d", fromId))},
			{Key: "toId", Value: []byte(fmt.Sprintf("%d", toId))},
			{Key: "seq", Value: []byte(fmt.Sprintf("%d", seq))},
		},
	}

	if err := p.privateWriter.WriteMessages(p.ctx, msg); err != nil {
		zap.S().Errorf("[SendPrivateChatMessage] 发送消息到Kafka失败 fromId=%d, toId=%d, seq=%d", fromId, toId, seq, zap.Error(err))
		return fmt.Errorf("发送消息到Kafka失败: %w", err)
	}

	zap.S().Debugf("[SendPrivateChatMessage] 单聊消息已发送到Kafka fromId=%d, toId=%d, seq=%d, messageId=%s", fromId, toId, seq, messageId)
	return nil
}

// Close 关闭生产者
func (p *MessageProducer) Close() error {
	var errs []error
	if p.groupWriter != nil {
		if err := p.groupWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭群聊writer失败: %w", err))
		}
	}
	if p.privateWriter != nil {
		if err := p.privateWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭单聊writer失败: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("关闭生产者时发生错误: %v", errs)
	}
	return nil
}

// GetDefaultProducer 获取默认生产者
func GetDefaultProducer() *MessageProducer {
	return defaultMessageProducer
}
