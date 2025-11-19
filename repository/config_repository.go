// repository/config_repository.go
package repository

import (
	"get_jobs_go/model"

	"gorm.io/gorm"
)

type ConfigRepository interface {
	FindAll() ([]*model.ConfigEntity, error)
	FindByKey(configKey string) (*model.ConfigEntity, error)
	FindByCategory(category string) ([]*model.ConfigEntity, error)
	Save(config *model.ConfigEntity) error
	Update(config *model.ConfigEntity) error
	Delete(id int64) error
}

type configRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) ConfigRepository {
	return &configRepository{db: db}
}

func (r *configRepository) FindAll() ([]*model.ConfigEntity, error) {
	var configs []*model.ConfigEntity
	err := r.db.Find(&configs).Error
	return configs, err
}

func (r *configRepository) FindByKey(configKey string) (*model.ConfigEntity, error) {
	var config model.ConfigEntity
	err := r.db.Where("config_key = ?", configKey).First(&config).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &config, err
}

func (r *configRepository) FindByCategory(category string) ([]*model.ConfigEntity, error) {
	var configs []*model.ConfigEntity
	err := r.db.Where("category = ?", category).Find(&configs).Error
	return configs, err
}

func (r *configRepository) Save(config *model.ConfigEntity) error {
	return r.db.Create(config).Error
}

func (r *configRepository) Update(config *model.ConfigEntity) error {
	return r.db.Save(config).Error
}

func (r *configRepository) Delete(id int64) error {
	return r.db.Delete(&model.ConfigEntity{}, id).Error
}