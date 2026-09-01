package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type SocialRepository struct{ db *gorm.DB }

func NewSocialRepository(db *gorm.DB) *SocialRepository { return &SocialRepository{db: db} }
func (r *SocialRepository) Friends(id uint) (rows []model.Friendship, err error) {
	err = r.db.Where("player_id = ? AND status = ?", id, "好友").Find(&rows).Error
	return
}
