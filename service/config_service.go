// service/config_service.go
package service

import (
	"get_jobs_go/model"
	"get_jobs_go/config"
	"get_jobs_go/repository"
	"strings"
	"time"
)

type ConfigService struct {
	configRepo repository.ConfigRepository
	bossService *BossService
	// 其他平台的service可以后续添加
	// liepinService *LiepinService
	// zhilianService *ZhilianService  
	// job51Service *Job51Service
}

func NewConfigService(
	configRepo repository.ConfigRepository,
	bossService *BossService,
) *ConfigService {
	return &ConfigService{
		configRepo: configRepo,
		bossService: bossService,
	}
}

// GetAllConfigsAsMap 获取所有配置（以Map形式返回）
func (s *ConfigService) GetAllConfigsAsMap() (map[string]string, error) {
	configs, err := s.configRepo.FindAll()
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	for _, config := range configs {
		configMap[config.ConfigKey] = config.ConfigValue
	}

	return configMap, nil
}

// GetAllConfigs 获取所有配置
func (s *ConfigService) GetAllConfigs() ([]*model.ConfigEntity, error) {
	return s.configRepo.FindAll()
}

// GetConfigByKey 根据配置键获取配置
func (s *ConfigService) GetConfigByKey(configKey string) (*model.ConfigEntity, error) {
	return s.configRepo.FindByKey(configKey)
}

// GetConfigsByCategory 根据分类获取配置列表
func (s *ConfigService) GetConfigsByCategory(category string) ([]*model.ConfigEntity, error) {
	return s.configRepo.FindByCategory(category)
}

// GetConfigValue 根据配置键获取配置值（可能为null）
func (s *ConfigService) GetConfigValue(configKey string) (string, error) {
	entity, err := s.configRepo.FindByKey(configKey)
	if err != nil {
		return "", err
	}
	if entity == nil {
		return "", nil
	}
	return entity.ConfigValue, nil
}

// RequireConfigValue 根据配置键获取必填配置值（缺失或空则抛异常）
func (s *ConfigService) RequireConfigValue(configKey string) (string, error) {
	value, err := s.GetConfigValue(configKey)
	if err != nil {
		return "", err
	}
	if value == "" || strings.TrimSpace(value) == "" {
		return "", &ConfigRequiredError{ConfigKey: configKey}
	}
	return value, nil
}

// GetAiConfigs 获取AI调用所需的基础配置（BASE_URL, API_KEY, MODEL）
func (s *ConfigService) GetAiConfigs() (map[string]string, error) {
	result := make(map[string]string)
	
	baseUrl, err := s.RequireConfigValue("BASE_URL")
	if err != nil {
		return nil, err
	}
	
	apiKey, err := s.RequireConfigValue("API_KEY")
	if err != nil {
		return nil, err
	}
	
	model, err := s.RequireConfigValue("MODEL")
	if err != nil {
		return nil, err
	}
	
	result["BASE_URL"] = baseUrl
	result["API_KEY"] = apiKey
	result["MODEL"] = model
	
	return result, nil
}

// BatchUpdateConfigs 批量更新配置
func (s *ConfigService) BatchUpdateConfigs(configMap map[string]string) (int, error) {
	updateCount := 0

	for key, value := range configMap {
		config, err := s.configRepo.FindByKey(key)
		if err != nil {
			return updateCount, err
		}

		if config != nil {
			config.ConfigValue = value
			config.UpdatedAt = time.Now()
			if err := s.configRepo.Update(config); err != nil {
				return updateCount, err
			}
			updateCount++
			// 这里可以添加日志记录
			// log.Infof("更新配置: %s = %s", key, value)
		} else {
			// 这里可以添加警告日志
			// log.Warnf("配置键不存在: %s", key)
		}
	}

	return updateCount, nil
}

// UpdateConfig 更新单个配置
func (s *ConfigService) UpdateConfig(configKey, configValue string) (bool, error) {
	config, err := s.configRepo.FindByKey(configKey)
	if err != nil {
		return false, err
	}

	if config != nil {
		config.ConfigValue = configValue
		config.UpdatedAt = time.Now()
		if err := s.configRepo.Update(config); err != nil {
			return false, err
		}
		// 这里可以添加日志记录
		// log.Infof("更新配置成功: %s = %s", configKey, configValue)
		return true, nil
	} else {
		// 这里可以添加警告日志
		// log.Warnf("配置键不存在: %s", configKey)
		return false, nil
	}
}

// CreateConfig 创建新配置
func (s *ConfigService) CreateConfig(config *model.ConfigEntity) (bool, error) {
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now

	if err := s.configRepo.Save(config); err != nil {
		return false, err
	}

	// 这里可以添加日志记录
	// log.Infof("创建配置成功: %s = %s", config.ConfigKey, config.ConfigValue)
	return true, nil
}

// GetBossConfig 统一入口：获取Boss配置
func (s *ConfigService) GetBossConfig() (*config.BossConfig, error) {
	return s.bossService.LoadBossConfig()
}

// 其他平台的配置获取方法可以后续添加
/*
func (s *ConfigService) GetLiepinConfig() (*config.LiepinConfig, error) {
    return s.liepinService.LoadLiepinConfig()
}

func (s *ConfigService) GetZhilianConfig() (*config.ZhilianConfig, error) {
    return s.zhilianService.LoadZhilianConfig()
}

func (s *ConfigService) GetJob51Config() (*config.Job51Config, error) {
    return s.job51Service.LoadJob51Config()
}
*/

// ConfigRequiredError 配置缺失错误
type ConfigRequiredError struct {
	ConfigKey string
}

func (e *ConfigRequiredError) Error() string {
	return "缺少必要配置: " + e.ConfigKey
}