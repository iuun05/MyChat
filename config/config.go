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
	Brokers            []string `mapstructure:"brokers" json:"brokers"`                           // Kafka broker地址列表
	GroupTopic         string   `mapstructure:"group_topic" json:"group_topic"`                   // 群聊消息topic
	PrivateTopic       string   `mapstructure:"private_topic" json:"private_topic"`               // 单聊消息topic
	GroupID            string   `mapstructure:"group_id" json:"group_id"`                         // 群聊消费者组ID
	PrivateGroupID     string   `mapstructure:"private_group_id" json:"private_group_id"`         // 单聊消费者组ID
	WorkerCount        int      `mapstructure:"worker_count" json:"worker_count"`                 // 消费者worker数量
	PrivateWorkerCount int      `mapstructure:"private_worker_count" json:"private_worker_count"` // 单聊消费者worker数量
}

type ClusterConfig struct {
	Enabled           bool   `mapstructure:"enabled" json:"enabled"`
	NodeID            string `mapstructure:"node_id" json:"node_id"`
	ChannelPrefix     string `mapstructure:"channel_prefix" json:"channel_prefix"`
	UserNodePrefix    string `mapstructure:"user_node_prefix" json:"user_node_prefix"`
	BindingTTLSeconds int    `mapstructure:"binding_ttl_seconds" json:"binding_ttl_seconds"`
}

type ServiceConfig struct {
	Port     int           `mapstructure:"port" json:"port"`
	GRPCPort int           `mapstructure:"grpc_port" json:"grpc_port"`
	DB       MysqlConfig   `mapstructure:"mysql" json:"mysql"`
	RedisDB  RedisConfig   `mapstructure:"redis" json:"redis"`
	Kafka    KafkaConfig   `mapstructure:"kafka" json:"kafka"`
	Cluster  ClusterConfig `mapstructure:"cluster" json:"cluster"`
}
