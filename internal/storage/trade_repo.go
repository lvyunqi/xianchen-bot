package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type TradeRepository struct{ db *gorm.DB }

func NewTradeRepository(db *gorm.DB) *TradeRepository { return &TradeRepository{db: db} }
func (r *TradeRepository) Active() (rows []model.TradeListing, err error) {
	err = r.db.Where("status = ?", "上架").Order("id DESC").Find(&rows).Error
	return
}
