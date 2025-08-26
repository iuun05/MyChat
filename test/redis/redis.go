package main

import (
	"MyChat/global"
	"MyChat/models"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 初始化 redis 连接
	global.RedisDB = redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   0,
	})

	// 启动 WebSocket 服务
	http.HandleFunc("/chat", models.Chat)
	go func() {
		log.Println("Server started at :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 等待服务启动
	time.Sleep(2 * time.Second)

	// 模拟两个客户端
	go mockClient(1, 2) // userId=1
	go mockClient(2, 1) // userId=2

	select {}
}

// 模拟客户端发送和接收
func mockClient(userId, targetId int64) {
	url := fmt.Sprintf("ws://127.0.0.1:8080/chat?userId=%d", userId)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial error:", err)
	}
	defer c.Close()

	// 发送一条消息
	msg := fmt.Sprintf(`{"userId":%d,"targetId":%d,"type":1,"content":"hello from %d"}`, userId, targetId, userId)
	err = c.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Println("send error:", err)
	}

	// 接收消息
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			return
		}
		log.Printf("user %d received: %s\n", userId, message)

		// 打印 redis 中的历史消息
		history := models.RedisMsg(userId, targetId, 0, 10, false)
		fmt.Printf("user %d history with %d: %v\n", userId, targetId, history)
	}
}
