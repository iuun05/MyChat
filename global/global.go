package global

import (
	"MyChat/config"

	"gorm.io/gorm"
)

var (
	DB            *gorm.DB
	ServiceConfig *config.ServiceConfig
)
