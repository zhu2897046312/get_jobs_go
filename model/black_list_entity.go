package model

import (
	"time"
)

// BlacklistEntity Boss黑名单实体类
type BlacklistEntity struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Type      string    `gorm:"column:type"`  // 类型：company(公司), recruiter(招聘者), job(职位)
	Value     string    `gorm:"column:value"` // 黑名单值
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (BlacklistEntity) TableName() string {
	return "boss_blacklist"
}