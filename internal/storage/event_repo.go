package storage

import (
	"gorm.io/gorm"
	"xianlv/internal/model"
)

type EventRepository struct{ db *gorm.DB }

func NewEventRepository(db *gorm.DB) *EventRepository { return &EventRepository{db: db} }
func (r *EventRepository) List() (rows []model.Event, err error) {
	err = r.db.Order("type,name").Find(&rows).Error
	return
}
func (r *EventRepository) Get(id uint) (row model.Event, err error) {
	err = r.db.First(&row, id).Error
	return
}
func (r *EventRepository) Create(row *model.Event) error { return r.db.Create(row).Error }
func (r *EventRepository) Update(id uint, changes map[string]any) error {
	return r.db.Model(&model.Event{}).Where("id = ?", id).Updates(changes).Error
}
func (r *EventRepository) Delete(id uint) error { return r.db.Delete(&model.Event{}, id).Error }
