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

// ListReceivedPaged 按收件人+类型分页读信笺（diary 等自发自收场景传相同 ID）。
func (r *SocialRepository) ListReceivedPaged(receiverID uint, typ string, page, pageSize int) (rows []model.SocialMessage, total int64, err error) {
	query := r.db.Model(&model.SocialMessage{}).
		Where("receiver_id = ? AND type = ?", receiverID, typ)
	if err = query.Count(&total).Error; err != nil {
		return
	}
	err = query.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&rows).Error
	return
}

// CountUnread 按收件人统计一组类型的未读数。
func (r *SocialRepository) CountUnread(receiverID uint, types []string) (int64, error) {
	var unread int64
	err := r.db.Model(&model.SocialMessage{}).
		Where("receiver_id = ? AND type IN ? AND read = ?", receiverID, types, false).
		Count(&unread).Error
	return unread, err
}

// ListNotificationsPaged 系统通知分页（含未读数），通知类型由调用方传入。
func (r *SocialRepository) ListNotificationsPaged(receiverID uint, types []string, page, pageSize int) (rows []model.SocialMessage, total, unread int64, err error) {
	base := r.db.Model(&model.SocialMessage{}).Where("receiver_id = ? AND type IN ?", receiverID, types)
	if err = base.Count(&total).Error; err != nil {
		return
	}
	if unread, err = r.CountUnread(receiverID, types); err != nil {
		return
	}
	err = base.Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&rows).Error
	return
}

// MarkReadByIDs 批量标记已读。
func (r *SocialRepository) MarkReadByIDs(ids []uint) error {
	return r.db.Model(&model.SocialMessage{}).Where("id IN ?", ids).Update("read", true).Error
}

// MarkTypeRead 按收件人+类型全量标记已读。
func (r *SocialRepository) MarkTypeRead(receiverID uint, typ string) error {
	return r.db.Model(&model.SocialMessage{}).
		Where("receiver_id = ? AND type = ? AND read = ?", receiverID, typ, false).
		Update("read", true).Error
}
