package model

import (
	"time"
)

// ConfigEntity 配置实体类
type ConfigEntity struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;column:id"`
	ConfigKey   string    `gorm:"column:config_key"`
	ConfigValue string    `gorm:"column:config_value"`
	ConfigType  string    `gorm:"column:config_type"`
	Category    string    `gorm:"column:category"`
	Description string    `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (ConfigEntity) TableName() string {
	return "config"
}