package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type ItemRepository struct{ db *gorm.DB }

func NewItemRepository(db *gorm.DB) *ItemRepository { return &ItemRepository{db: db} }
func (r *ItemRepository) List(offset, limit int) ([]model.Item, int64, error) {
	var rows []model.Item
	var total int64
	if err := r.db.Model(&model.Item{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}
func (r *ItemRepository) Get(id uint) (model.Item, error) {
	var row model.Item
	err := r.db.First(&row, id).Error
	return row, err
}
func (r *ItemRepository) Create(row *model.Item) error { return r.db.Create(row).Error }
func (r *ItemRepository) Update(id uint, changes map[string]any) error {
	return r.db.Model(&model.Item{}).Where("id = ?", id).Updates(changes).Error
}
func (r *ItemRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("item_id = ?", id).Delete(&model.DropEntry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("item_id = ?", id).Delete(&model.PlayerItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Item{}, id).Error
	})
}
