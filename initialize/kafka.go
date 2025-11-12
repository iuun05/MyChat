package initialize

import (
	"MyChat/dao"
	"MyChat/global"
	"MyChat/mq/kafka"

	"go.uber.org/zap"
)

var (
	kafkaProducer        *kafka.MessageProducer
	groupKafkaConsumer   *kafka.MessageConsumer
	privateKafkaConsumer *kafka.MessageConsumer
)

// InitKafka 初始化Kafka生产者和消费者
func InitKafka() {
	cfg := global.ServiceConfig.Kafka

	// 检查配置
	if len(cfg.Brokers) == 0 {
		zap.S().Warn("[InitKafka] Kafka配置为空，跳过初始化")
		return
	}

	if cfg.GroupTopic == "" || cfg.PrivateTopic == "" {
		zap.S().Warn("[InitKafka] Kafka topic配置不完整，跳过初始化")
		return
	}

	// 初始化生产者（支持群聊和单聊两个topic）
	kafkaProducer = kafka.NewMessageProducer(cfg.Brokers, cfg.GroupTopic, cfg.PrivateTopic)
	if kafkaProducer == nil {
		zap.S().Error("[InitKafka] Kafka生产者初始化失败")
		return
	}

	// 设置到MessageDAO
	messageDAO := dao.NewMessageDAO()
	messageDAO.SetKafkaProducer(kafkaProducer)

	zap.S().Info("[InitKafka] Kafka生产者初始化成功")

	// 初始化群聊消费者
	if cfg.GroupID != "" && cfg.WorkerCount > 0 {
		groupKafkaConsumer = kafka.NewMessageConsumer(
			cfg.Brokers,
			cfg.GroupTopic,
			cfg.GroupID,
			messageDAO,
			cfg.WorkerCount,
		)

		// 启动群聊消费者
		groupKafkaConsumer.Start()
		zap.S().Infof("[InitKafka] 群聊Kafka消费者启动成功 workers=%d", cfg.WorkerCount)
	} else {
		zap.S().Warn("[InitKafka] 群聊Kafka消费者配置不完整，跳过初始化")
	}

	// 初始化单聊消费者
	if cfg.PrivateGroupID != "" && cfg.PrivateWorkerCount > 0 {
		privateKafkaConsumer = kafka.NewMessageConsumer(
			cfg.Brokers,
			cfg.PrivateTopic,
			cfg.PrivateGroupID,
			messageDAO,
			cfg.PrivateWorkerCount,
		)

		// 启动单聊消费者
		privateKafkaConsumer.Start()
		zap.S().Infof("[InitKafka] 单聊Kafka消费者启动成功 workers=%d", cfg.PrivateWorkerCount)
	} else {
		zap.S().Warn("[InitKafka] 单聊Kafka消费者配置不完整，跳过初始化")
	}
}

// CloseKafka 关闭Kafka连接
func CloseKafka() {
	// 关闭群聊消费者
	if groupKafkaConsumer != nil {
		if err := groupKafkaConsumer.Stop(); err != nil {
			zap.S().Errorf("[CloseKafka] 关闭群聊消费者失败", zap.Error(err))
		}
	}

	// 关闭单聊消费者
	if privateKafkaConsumer != nil {
		if err := privateKafkaConsumer.Stop(); err != nil {
			zap.S().Errorf("[CloseKafka] 关闭单聊消费者失败", zap.Error(err))
		}
	}

	// 关闭生产者
	if kafkaProducer != nil {
		if err := kafkaProducer.Close(); err != nil {
			zap.S().Errorf("[CloseKafka] 关闭生产者失败", zap.Error(err))
		}
	}

	zap.S().Info("[CloseKafka] Kafka连接已关闭")
}

// GetKafkaProducer 获取Kafka生产者
func GetKafkaProducer() *kafka.MessageProducer {
	return kafkaProducer
}

// GetGroupKafkaConsumer 获取群聊Kafka消费者
func GetGroupKafkaConsumer() *kafka.MessageConsumer {
	return groupKafkaConsumer
}

// GetPrivateKafkaConsumer 获取单聊Kafka消费者
func GetPrivateKafkaConsumer() *kafka.MessageConsumer {
	return privateKafkaConsumer
}
