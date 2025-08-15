package initialize

import (
	"MyChat/global"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func InitConfig() {
	v := viper.New()

	ConfigFilePath := "config/config.yaml"

	v.SetConfigFile(ConfigFilePath)

	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}

	if err := v.Unmarshal(&global.ServiceConfig); err != nil {
		panic(err)
	}

	zap.S().Info("配置信息", global.ServiceConfig)
	// fmt.Printf("%v", global.ServiceConfig)
}
