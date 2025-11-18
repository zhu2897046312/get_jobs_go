package model

import (
	"time"
)


// CookieEntity Cookie实体类
type CookieEntity struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;column:id"`
	Platform   string    `gorm:"column:platform"`    // 平台名称（boss/zhilian/job51/liepin）
	CookieValue string   `gorm:"column:cookie_value"` // Cookie值
	Remark     string    `gorm:"column:remark"`      // 备注
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (CookieEntity) TableName() string {
	return "cookie"
}