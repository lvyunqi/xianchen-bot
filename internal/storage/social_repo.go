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

// Create 在仓库自身连接上写入一条社交消息。私信/通知是玩家即时可见数据，
// 不做应用层缓冲，逐条落库保证可见性；历史堆积由 retention 策略（60 天）收敛。
func (r *SocialRepository) Create(msg *model.SocialMessage) error {
	return r.db.Create(msg).Error
}

// CreateInTx 在调用方事务内写入，保证消息与业务状态同时生效或同时回滚。
func (r *SocialRepository) CreateInTx(tx *gorm.DB, msg *model.SocialMessage) error {
	return tx.Create(msg).Error
}
