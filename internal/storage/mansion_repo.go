package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type MansionRepository struct{ db *gorm.DB }

func NewMansionRepository(db *gorm.DB) *MansionRepository { return &MansionRepository{db: db} }
func (r *MansionRepository) GetByPlayer(id uint) (row model.Mansion, err error) {
	err = r.db.Where("player_id = ?", id).First(&row).Error
	return
}
