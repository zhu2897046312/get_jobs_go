package model

import (
	"time"
)

// AiEntity AI配置实体类
type AiEntity struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Introduce string    `gorm:"column:introduce"`
	Prompt    string    `gorm:"column:prompt"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (AiEntity) TableName() string {
	return "ai"
}


