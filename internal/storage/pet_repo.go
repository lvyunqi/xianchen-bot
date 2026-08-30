package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type PetRepository struct{ db *gorm.DB }

func NewPetRepository(db *gorm.DB) *PetRepository { return &PetRepository{db: db} }
func (r *PetRepository) ListByPlayer(id uint) (rows []model.Pet, err error) {
	err = r.db.Where("player_id = ?", id).Find(&rows).Error
	return
}
