package initialize

import (
	"MyChat/dao"
	"MyChat/global"
	"MyChat/mq/kafka"

	"go.uber.org/zap"
)

var (
	kafkaProducer *kafka.MessageProducer
	kafkaConsumer *kafka.MessageConsumer
)

// InitKafka 初始化Kafka生产者和消费者
func InitKafka() {
	cfg := global.ServiceConfig.Kafka

	// 检查配置
	if len(cfg.Brokers) == 0 {
		zap.S().Warn("[InitKafka] Kafka配置为空，跳过初始化")
		return
	}

	if cfg.Topic == "" {
		zap.S().Warn("[InitKafka] Kafka topic为空，跳过初始化")
		return
	}

	// 初始化生产者
	kafkaProducer = kafka.NewMessageProducer(cfg.Brokers, cfg.Topic)
	if kafkaProducer == nil {
		zap.S().Error("[InitKafka] Kafka生产者初始化失败")
		return
	}

	// 设置到MessageDAO
	messageDAO := dao.NewMessageDAO()
	messageDAO.SetKafkaProducer(kafkaProducer)

	zap.S().Info("[InitKafka] Kafka生产者初始化成功")

	// 初始化消费者
	if cfg.GroupID != "" && cfg.WorkerCount > 0 {
		kafkaConsumer = kafka.NewMessageConsumer(
			cfg.Brokers,
			cfg.Topic,
			cfg.GroupID,
			messageDAO,
			cfg.WorkerCount,
		)

		// 启动消费者
		kafkaConsumer.Start()
		zap.S().Infof("[InitKafka] Kafka消费者启动成功 workers=%d", cfg.WorkerCount)
	} else {
		zap.S().Warn("[InitKafka] Kafka消费者配置不完整，跳过初始化")
	}
}

// CloseKafka 关闭Kafka连接
func CloseKafka() {
	if kafkaConsumer != nil {
		if err := kafkaConsumer.Stop(); err != nil {
			zap.S().Errorf("[CloseKafka] 关闭消费者失败", zap.Error(err))
		}
	}

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

// GetKafkaConsumer 获取Kafka消费者
func GetKafkaConsumer() *kafka.MessageConsumer {
	return kafkaConsumer
}
