package repository

import (
	"get_jobs_go/model"
	"log"

	"gorm.io/gorm"
)

// AiRepository AI配置仓储接口
type AiRepository interface {
	FindAll() ([]*model.AiEntity, error)
	FindByID(id int64) (*model.AiEntity, error)
	Save(ai *model.AiEntity) error
	Update(ai *model.AiEntity) error
	Delete(id int64) error
	FindLatest() (*model.AiEntity, error)
}

type aiRepository struct {
	db *gorm.DB
}

func NewAiRepository(db *gorm.DB) AiRepository {
	return &aiRepository{db: db}
}

func (r *aiRepository) FindAll() ([]*model.AiEntity, error) {
	var ais []*model.AiEntity
	result := r.db.Find(&ais)
	if result.Error != nil {
		return nil, result.Error
	}
	return ais, nil
}

func (r *aiRepository) FindByID(id int64) (*model.AiEntity, error) {
	var ai model.AiEntity
	result := r.db.First(&ai, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &ai, nil
}

func (r *aiRepository) Save(ai *model.AiEntity) error {
	result := r.db.Create(ai)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("创建新的AI配置，ID: %d", ai.ID)
	return nil
}

func (r *aiRepository) Update(ai *model.AiEntity) error {
	result := r.db.Save(ai)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("更新AI配置，ID: %d", ai.ID)
	return nil
}

func (r *aiRepository) Delete(id int64) error {
	result := r.db.Delete(&model.AiEntity{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("删除AI配置成功，ID: %d", id)
	}
	return nil
}

func (r *aiRepository) FindLatest() (*model.AiEntity, error) {
	var ai model.AiEntity
	result := r.db.Order("id DESC").First(&ai)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &ai, nil
}