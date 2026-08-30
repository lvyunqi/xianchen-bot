package storage

import (
	"gorm.io/gorm"
	"time"
	"xianlv/internal/model"
)

type RankRepository struct{ db *gorm.DB }

func NewRankRepository(db *gorm.DB) *RankRepository { return &RankRepository{db: db} }
func (r *RankRepository) Refresh() error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&model.RankEntry{}).Error; err != nil {
			return err
		}
		types := []struct{ name, field string }{{"修为榜", "cultivation"}, {"战力榜", "combat_power"}, {"功德榜", "merit"}}
		now := time.Now()
		for _, kind := range types {
			var players []model.Player
			if err := tx.Order(kind.field + " DESC").Limit(100).Find(&players).Error; err != nil {
				return err
			}
			for i, p := range players {
				score := p.Cultivation
				if kind.field == "combat_power" {
					score = p.CombatPower
				} else if kind.field == "merit" {
					score = p.Merit
				}
				if err := tx.Create(&model.RankEntry{Type: kind.name, PlayerID: p.ID, PlayerName: p.DaoName, Rank: i + 1, Score: score, RefreshedAt: now}).Error; err != nil {
					return err
				}
			}
		}
		var couples []model.Couple
		if err := tx.Where("status = ?", model.CoupleStatusActive).Order("affinity DESC").Limit(100).Find(&couples).Error; err != nil {
			return err
		}
		for i, c := range couples {
			if err := tx.Create(&model.RankEntry{Type: "道缘榜", PlayerID: c.ID, PlayerName: c.PlayerAName + " · " + c.PlayerBName, Rank: i + 1, Score: c.Affinity, RefreshedAt: now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (r *RankRepository) List(kind string) (rows []model.RankEntry, err error) {
	err = r.db.Where("type = ?", kind).Order("rank").Find(&rows).Error
	return
}
