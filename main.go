package main

import (
	"MyChat/initialize"
	"MyChat/router"
)

func main() {
	// init logger
	initialize.InitLogger()

	// init config
	initialize.InitConfig()

	// init mysql
	initialize.InitDB()

	initialize.InitRedis()

	router := router.Router()
	router.Run(":8080")
}
