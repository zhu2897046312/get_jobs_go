package service

import (
	"get_jobs_go/model"
	"get_jobs_go/repository"
	"time"
)

// CookieService Cookie服务
type CookieService struct {
	cookieRepo repository.CookieRepository
}

func NewCookieService(cookieRepo repository.CookieRepository) *CookieService {
	return &CookieService{
		cookieRepo: cookieRepo,
	}
}

// GetCookieByPlatform 根据平台获取Cookie
func (s *CookieService) GetCookieByPlatform(platform string) (*model.CookieEntity, error) {
	return s.cookieRepo.FindByPlatform(platform)
}

// SaveOrUpdateCookie 保存或更新Cookie
func (s *CookieService) SaveOrUpdateCookie(platform, cookieValue, remark string) (bool, error) {
	existingCookie, err := s.cookieRepo.FindByPlatform(platform)
	if err != nil {
		return false, err
	}

	now := time.Now()

	if existingCookie != nil {
		// 更新现有Cookie
		existingCookie.CookieValue = cookieValue
		existingCookie.Remark = remark
		existingCookie.UpdatedAt = now
		if err := s.cookieRepo.Update(existingCookie); err != nil {
			return false, err
		}
		return true, nil
	} else {
		// 新建Cookie
		newCookie := &model.CookieEntity{
			Platform:    platform,
			CookieValue: cookieValue,
			Remark:      remark,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.cookieRepo.Save(newCookie); err != nil {
			return false, err
		}
		return true, nil
	}
}

// ClearCookieByPlatform 清空指定平台的所有Cookie值
func (s *CookieService) ClearCookieByPlatform(platform, remark string) (bool, error) {
	if err := s.cookieRepo.ClearCookieValue(platform, remark); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteCookie 删除指定平台的Cookie
func (s *CookieService) DeleteCookie(platform string) (bool, error) {
	if err := s.cookieRepo.DeleteByPlatform(platform); err != nil {
		return false, err
	}
	return true, nil
}

// GetAllCookies 获取所有Cookie
func (s *CookieService) GetAllCookies() ([]*model.CookieEntity, error) {
	return s.cookieRepo.FindAll()
}

// GetCookieValueByPlatform 根据平台获取Cookie值（便捷方法）
func (s *CookieService) GetCookieValueByPlatform(platform string) (string, error) {
	cookie, err := s.cookieRepo.FindByPlatform(platform)
	if err != nil {
		return "", err
	}
	if cookie == nil {
		return "", nil
	}
	return cookie.CookieValue, nil
}

// GetPlatforms 获取所有支持的平台列表
func (s *CookieService) GetPlatforms() []string {
	return []string{"boss", "zhilian", "job51", "liepin"}
}

// ValidatePlatform 验证平台名称是否有效
func (s *CookieService) ValidatePlatform(platform string) bool {
	supportedPlatforms := s.GetPlatforms()
	for _, p := range supportedPlatforms {
		if p == platform {
			return true
		}
	}
	return false
}