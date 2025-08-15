package initialize

import (
	"log"

	"go.uber.org/zap"
)

func InitLogger() {
	// init logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("log init fail", err.Error())
	}

	zap.ReplaceGlobals(logger)
}
