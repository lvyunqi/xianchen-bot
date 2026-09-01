package storage

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"xianlv/internal/model"
)

type CoupleRepository struct{ db *gorm.DB }

func NewCoupleRepository(db *gorm.DB) *CoupleRepository { return &CoupleRepository{db: db} }
func (r *CoupleRepository) List(offset, limit int) ([]model.Couple, int64, error) {
	var rows []model.Couple
	var total int64
	db := r.db.Model(&model.Couple{}).Where("status = ?", model.CoupleStatusActive)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("affinity DESC,id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}
func (r *CoupleRepository) Get(id uint) (model.Couple, error) {
	var row model.Couple
	err := r.db.First(&row, id).Error
	return row, err
}
func (r *CoupleRepository) Update(id uint, changes map[string]any) error {
	return r.db.Model(&model.Couple{}).Where("id = ?", id).Updates(changes).Error
}
func (r *CoupleRepository) ForceBond(aID, bID uint) (model.Couple, error) {
	var result model.Couple
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if aID == bID {
			return errors.New("cannot bond player to self")
		}
		var a, b model.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, aID).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&b, bID).Error; err != nil {
			return err
		}
		if a.CoupleID != 0 || b.CoupleID != 0 {
			return errors.New("one of the players already has a couple")
		}
		result = model.Couple{PlayerAID: a.ID, PlayerBID: b.ID, PlayerAName: a.DaoName, PlayerBName: b.DaoName, Affinity: 0, BondLevel: 1, CultivationBonus: .3, Status: model.CoupleStatusActive, BondedAt: time.Now()}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		if err := tx.Model(&a).Update("couple_id", result.ID).Error; err != nil {
			return err
		}
		return tx.Model(&b).Update("couple_id", result.ID).Error
	})
	return result, err
}
func (r *CoupleRepository) ForceDissolve(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var row model.Couple
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return err
		}
		now := time.Now()
		if err := tx.Model(&row).Updates(map[string]any{"status": model.CoupleStatusDissolved, "dissolved_at": &now}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Player{}).Where("id IN ?", []uint{row.PlayerAID, row.PlayerBID}).Update("couple_id", 0).Error
	})
}
