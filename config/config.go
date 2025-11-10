package config

// MysqlConfig mysql信息配置
type MysqlConfig struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Name     string `mapstructure:"name" json:"Name"`
	User     string `mapstructure:"user" json:"user"`
	Password string `mapstructure:"password" json:"password"`
}

type RedisConfig struct {
	Host string `mapstructure:"host" json:"host"`
	Port int    `mapstructure:"port" json:"port"`
}

type KafkaConfig struct {
	Brokers     []string `mapstructure:"brokers" json:"brokers"`           // Kafka broker地址列表
	Topic       string   `mapstructure:"topic" json:"topic"`               // 群聊消息topic
	GroupID     string   `mapstructure:"group_id" json:"group_id"`         // 消费者组ID
	WorkerCount int      `mapstructure:"worker_count" json:"worker_count"` // 消费者worker数量
}

type ServiceConfig struct {
	Port     int         `mapstructure:"port" json:"port"`
	GRPCPort int         `mapstructure:"grpc_port" json:"grpc_port"`
	DB       MysqlConfig `mapstructure:"mysql" json:"mysql"`
	RedisDB  RedisConfig `mapstructure:"redis" json:"redis"`
	Kafka    KafkaConfig `mapstructure:"kafka" json:"kafka"`
}
