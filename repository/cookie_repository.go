package repository

import (
	"get_jobs_go/model"
	"log"
	"time"

	"gorm.io/gorm"
)

// CookieRepository Cookie仓储接口
type CookieRepository interface {
	FindByPlatform(platform string) (*model.CookieEntity, error)
	FindAll() ([]*model.CookieEntity, error)
	Save(cookie *model.CookieEntity) error
	Update(cookie *model.CookieEntity) error
	DeleteByPlatform(platform string) error
	ClearCookieValue(platform, remark string) error
}

type cookieRepository struct {
	db *gorm.DB
}

func NewCookieRepository(db *gorm.DB) CookieRepository {
	return &cookieRepository{db: db}
}

// FindByPlatform 根据平台获取Cookie（获取最新的一条）
func (r *cookieRepository) FindByPlatform(platform string) (*model.CookieEntity, error) {
	var cookie model.CookieEntity
	result := r.db.Where("platform = ?", platform).
		Order("updated_at DESC").
		First(&cookie)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &cookie, nil
}

// FindAll 获取所有Cookie
func (r *cookieRepository) FindAll() ([]*model.CookieEntity, error) {
	var cookies []*model.CookieEntity
	result := r.db.Find(&cookies)
	if result.Error != nil {
		return nil, result.Error
	}
	return cookies, nil
}

// Save 保存Cookie
func (r *cookieRepository) Save(cookie *model.CookieEntity) error {
	result := r.db.Create(cookie)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("创建Cookie成功: platform=%s", cookie.Platform)
	return nil
}

// Update 更新Cookie
func (r *cookieRepository) Update(cookie *model.CookieEntity) error {
	result := r.db.Save(cookie)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("更新Cookie成功: platform=%s", cookie.Platform)
	return nil
}

// DeleteByPlatform 删除指定平台的Cookie
func (r *cookieRepository) DeleteByPlatform(platform string) error {
	result := r.db.Where("platform = ?", platform).Delete(&model.CookieEntity{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("删除Cookie成功: platform=%s", platform)
	}
	return nil
}

// ClearCookieValue 清空指定平台的Cookie值
func (r *cookieRepository) ClearCookieValue(platform, remark string) error {
	result := r.db.Model(&model.CookieEntity{}).
		Where("platform = ?", platform).
		Updates(map[string]interface{}{
			"cookie_value": "",
			"remark":       remark,
			"updated_at":   time.Now(),
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected > 0 {
		log.Printf("清空Cookie值成功: platform=%s", platform)
	}
	return nil
}