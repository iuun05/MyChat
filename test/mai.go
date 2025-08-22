package main

// func main() {
// 	dsn := "root:@tcp(127.0.0.1:3306)/MyChat?charset=utf8mb4&parseTime=True&loc=Local"

// 	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
// 	if err != nil {
// 		panic(err)
// 	}

// 	// err = db.AutoMigrate(&models.User_basic{})
// 	// err = db.AutoMigrate(&models.Relation{})
// 	err = db.AutoMigrate(&models.Message{})
// 	if err != nil {
// 		panic(err)
// 	}
// }
