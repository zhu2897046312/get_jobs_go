package repository

import (
	"get_jobs_go/model"
	"time"

	"gorm.io/gorm"
)

// BossOptionRepository Boss选项仓储接口 
type BossOptionRepository interface {
	FindByType(typeStr string) ([]*model.BossOptionEntity, error)
	FindAll() ([]*model.BossOptionEntity, error)
	FindByTypeAndCode(typeStr, code string) (*model.BossOptionEntity, error)
	Save(option *model.BossOptionEntity) error
	Update(option *model.BossOptionEntity) error
	Delete(id int64) error
}

type bossOptionRepository struct {
	db *gorm.DB
}

func NewBossOptionRepository(db *gorm.DB) BossOptionRepository {
	return &bossOptionRepository{db: db}
}

func (r *bossOptionRepository) FindByType(typeStr string) ([]*model.BossOptionEntity, error) {
	var options []*model.BossOptionEntity
	
	// 对于city和industry类型，需要特殊排序处理
	if typeStr == "city" || typeStr == "industry" {
		result := r.db.Where("type = ?", typeStr).
			Order("sort_order IS NULL, sort_order ASC, id ASC").
			Find(&options)
		if result.Error != nil {
			return nil, result.Error
		}
	} else {
		result := r.db.Where("type = ?", typeStr).Order("id ASC").Find(&options)
		if result.Error != nil {
			return nil, result.Error
		}
	}
	
	return options, nil
}

func (r *bossOptionRepository) FindAll() ([]*model.BossOptionEntity, error) {
	var options []*model.BossOptionEntity
	result := r.db.Find(&options)
	if result.Error != nil {
		return nil, result.Error
	}
	return options, nil
}

func (r *bossOptionRepository) FindByTypeAndCode(typeStr, code string) (*model.BossOptionEntity, error) {
	var option model.BossOptionEntity
	result := r.db.Where("type = ? AND code = ?", typeStr, code).First(&option)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &option, nil
}

func (r *bossOptionRepository) Save(option *model.BossOptionEntity) error {
	result := r.db.Create(option)
	return result.Error
}

func (r *bossOptionRepository) Update(option *model.BossOptionEntity) error {
	result := r.db.Save(option)
	return result.Error
}

func (r *bossOptionRepository) Delete(id int64) error {
	result := r.db.Delete(&model.BossOptionEntity{}, id)
	return result.Error
}

// BossIndustryRepository Boss行业仓储接口
type BossIndustryRepository interface {
	FindAll() ([]*model.BossIndustryEntity, error)
	FindByCode(code int) (*model.BossIndustryEntity, error)
	FindByName(name string) (*model.BossIndustryEntity, error)
	Save(industry *model.BossIndustryEntity) error
	Update(industry *model.BossIndustryEntity) error
	Delete(id int64) error
}

type bossIndustryRepository struct {
	db *gorm.DB
}

func NewBossIndustryRepository(db *gorm.DB) BossIndustryRepository {
	return &bossIndustryRepository{db: db}
}

func (r *bossIndustryRepository) FindAll() ([]*model.BossIndustryEntity, error) {
	var industries []*model.BossIndustryEntity
	result := r.db.Find(&industries)
	if result.Error != nil {
		return nil, result.Error
	}
	return industries, nil
}

func (r *bossIndustryRepository) FindByCode(code int) (*model.BossIndustryEntity, error) {
	var industry model.BossIndustryEntity
	result := r.db.Where("code = ?", code).First(&industry)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &industry, nil
}

func (r *bossIndustryRepository) FindByName(name string) (*model.BossIndustryEntity, error) {
	var industry model.BossIndustryEntity
	result := r.db.Where("name = ?", name).First(&industry)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &industry, nil
}

func (r *bossIndustryRepository) Save(industry *model.BossIndustryEntity) error {
	result := r.db.Create(industry)
	return result.Error
}

func (r *bossIndustryRepository) Update(industry *model.BossIndustryEntity) error {
	result := r.db.Save(industry)
	return result.Error
}

func (r *bossIndustryRepository) Delete(id int64) error {
	result := r.db.Delete(&model.BossIndustryEntity{}, id)
	return result.Error
}

// BossConfigRepository Boss配置仓储接口
type BossConfigRepository interface {
	FindAll() ([]*model.BossConfigEntity, error)
	FindByID(id int64) (*model.BossConfigEntity, error)
	FindFirst() (*model.BossConfigEntity, error)
	Save(config *model.BossConfigEntity) error
	Update(config *model.BossConfigEntity) error
	Delete(id int64) error
}

type bossConfigRepository struct {
	db *gorm.DB
}

func NewBossConfigRepository(db *gorm.DB) BossConfigRepository {
	return &bossConfigRepository{db: db}
}

func (r *bossConfigRepository) FindAll() ([]*model.BossConfigEntity, error) {
	var configs []*model.BossConfigEntity
	result := r.db.Find(&configs)
	if result.Error != nil {
		return nil, result.Error
	}
	return configs, nil
}

func (r *bossConfigRepository) FindByID(id int64) (*model.BossConfigEntity, error) {
	var config model.BossConfigEntity
	result := r.db.First(&config, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &config, nil
}

func (r *bossConfigRepository) FindFirst() (*model.BossConfigEntity, error) {
	var config model.BossConfigEntity
	result := r.db.First(&config)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &config, nil
}

func (r *bossConfigRepository) Save(config *model.BossConfigEntity) error {
	result := r.db.Create(config)
	return result.Error
}

func (r *bossConfigRepository) Update(config *model.BossConfigEntity) error {
	result := r.db.Save(config)
	return result.Error
}

func (r *bossConfigRepository) Delete(id int64) error {
	result := r.db.Delete(&model.BossConfigEntity{}, id)
	return result.Error
}

// BlacklistRepository 黑名单仓储接口
type BlacklistRepository interface {
	FindByType(typeStr string) ([]*model.BlacklistEntity, error)
	FindAll() ([]*model.BlacklistEntity, error)
	Save(blacklist *model.BlacklistEntity) error
	DeleteByTypeAndValue(typeStr, value string) error
	CountByTypeAndValue(typeStr, value string) (int64, error)
}

type blacklistRepository struct {
	db *gorm.DB
}

func NewBlacklistRepository(db *gorm.DB) BlacklistRepository {
	return &blacklistRepository{db: db}
}

func (r *blacklistRepository) FindByType(typeStr string) ([]*model.BlacklistEntity, error) {
	var blacklists []*model.BlacklistEntity
	result := r.db.Where("type = ?", typeStr).Find(&blacklists)
	if result.Error != nil {
		return nil, result.Error
	}
	return blacklists, nil
}

func (r *blacklistRepository) FindAll() ([]*model.BlacklistEntity, error) {
	var blacklists []*model.BlacklistEntity
	result := r.db.Find(&blacklists)
	if result.Error != nil {
		return nil, result.Error
	}
	return blacklists, nil
}

func (r *blacklistRepository) Save(blacklist *model.BlacklistEntity) error {
	result := r.db.Create(blacklist)
	return result.Error
}

func (r *blacklistRepository) DeleteByTypeAndValue(typeStr, value string) error {
	result := r.db.Where("type = ? AND value = ?", typeStr, value).Delete(&model.BlacklistEntity{})
	return result.Error
}

func (r *blacklistRepository) CountByTypeAndValue(typeStr, value string) (int64, error) {
	var count int64
	result := r.db.Model(&model.BlacklistEntity{}).Where("type = ? AND value = ?", typeStr, value).Count(&count)
	return count, result.Error
}

// BossJobDataRepository Boss职位数据仓储接口
type BossJobDataRepository interface {
	FindAll() ([]*model.BossJobDataEntity, error)
	FindByEncryptIdAndUserId(encryptId, encryptUserId string) (*model.BossJobDataEntity, error)
	FindByEncryptId(encryptId string) (*model.BossJobDataEntity, error)
	Save(job *model.BossJobDataEntity) error
	Update(job *model.BossJobDataEntity) error
	UpdateDeliveryStatus(encryptId, encryptUserId, status string) error
	CountByCondition(condition string, args ...interface{}) (int64, error)
	FindByWrapper(wrapper *gorm.DB) ([]*model.BossJobDataEntity, error)
	CountByWrapper(wrapper *gorm.DB) (int64, error)
}

type bossJobDataRepository struct {
	db *gorm.DB
}

func NewBossJobDataRepository(db *gorm.DB) BossJobDataRepository {
	return &bossJobDataRepository{db: db}
}

func (r *bossJobDataRepository) FindAll() ([]*model.BossJobDataEntity, error) {
	var jobs []*model.BossJobDataEntity
	result := r.db.Find(&jobs)
	if result.Error != nil {
		return nil, result.Error
	}
	return jobs, nil
}

func (r *bossJobDataRepository) FindByEncryptIdAndUserId(encryptId, encryptUserId string) (*model.BossJobDataEntity, error) {
	var job model.BossJobDataEntity
	result := r.db.Where("encrypt_id = ? AND encrypt_user_id = ?", encryptId, encryptUserId).First(&job)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &job, nil
}

func (r *bossJobDataRepository) FindByEncryptId(encryptId string) (*model.BossJobDataEntity, error) {
	var job model.BossJobDataEntity
	result := r.db.Where("encrypt_id = ?", encryptId).First(&job)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &job, nil
}

func (r *bossJobDataRepository) Save(job *model.BossJobDataEntity) error {
	result := r.db.Create(job)
	return result.Error
}

func (r *bossJobDataRepository) Update(job *model.BossJobDataEntity) error {
	result := r.db.Save(job)
	return result.Error
}

func (r *bossJobDataRepository) UpdateDeliveryStatus(encryptId, encryptUserId, status string) error {
	result := r.db.Model(&model.BossJobDataEntity{}).
		Where("encrypt_id = ?", encryptId).
		Where("encrypt_user_id = ?", encryptUserId).
		Updates(map[string]interface{}{
			"delivery_status": status,
			"updated_at":      time.Now(),
		})
	return result.Error
}

func (r *bossJobDataRepository) CountByCondition(condition string, args ...interface{}) (int64, error) {
	var count int64
	result := r.db.Model(&model.BossJobDataEntity{}).Where(condition, args...).Count(&count)
	return count, result.Error
}

func (r *bossJobDataRepository) FindByWrapper(wrapper *gorm.DB) ([]*model.BossJobDataEntity, error) {
	var jobs []*model.BossJobDataEntity
	result := wrapper.Find(&jobs)
	if result.Error != nil {
		return nil, result.Error
	}
	return jobs, nil
}

func (r *bossJobDataRepository) CountByWrapper(wrapper *gorm.DB) (int64, error) {
	var count int64
	result := wrapper.Count(&count)
	return count, result.Error
}