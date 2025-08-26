package dao

import (
	"MyChat/global"
	"MyChat/models"
	"fmt"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setupBenchmark() {
	dsn := "root:@tcp(127.0.0.1:3306)/MyChat?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	global.DB = db

	err = db.AutoMigrate(&models.UserBasic{}, &models.Relation{}, &models.Community{})
	if err != nil {
		panic(err)
	}
}

func BenchmarkCreateUser(b *testing.B) {
	setupBenchmark()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := models.UserBasic{
			Name:     fmt.Sprintf("user%d", i),
			PassWord: "password",
			Email:    fmt.Sprintf("user%d@example.com", i),
		}
		CreateUser(user)
	}
}

func BenchmarkFindUserByName(b *testing.B) {
	setupBenchmark()

	// 预创建一些用户
	for i := 0; i < 1000; i++ {
		user := models.UserBasic{
			Name:     fmt.Sprintf("user%d", i),
			PassWord: "password",
		}
		CreateUser(user)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindUserByName(fmt.Sprintf("user%d", i%1000))
	}
}

func BenchmarkAddFriend(b *testing.B) {
	setupBenchmark()

	// 预创建用户
	users := make([]*models.UserBasic, 100)
	for i := 0; i < 100; i++ {
		user := models.UserBasic{
			Name:     fmt.Sprintf("user%d", i),
			PassWord: "password",
		}
		created, _ := CreateUser(user)
		users[i] = created
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userA := users[i%100]
		userB := users[(i+1)%100]
		if userA.ID != userB.ID {
			AddFriend(userA.ID, userB.ID)
		}
	}
}
