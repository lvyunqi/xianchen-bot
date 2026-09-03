package storage

import (
	"gorm.io/gorm"
	"time"
	"xianlv/internal/model"
)

type TaskRepository struct{ db *gorm.DB }

func NewTaskRepository(db *gorm.DB) *TaskRepository { return &TaskRepository{db: db} }
func (r *TaskRepository) List() (rows []model.TaskTemplate, err error) {
	err = r.db.Order("type,name").Find(&rows).Error
	return
}
func (r *TaskRepository) Get(id uint) (row model.TaskTemplate, err error) {
	err = r.db.First(&row, id).Error
	return
}
func (r *TaskRepository) Create(row *model.TaskTemplate) error { return r.db.Create(row).Error }
func (r *TaskRepository) Update(id uint, changes map[string]any) error {
	return r.db.Model(&model.TaskTemplate{}).Where("id = ?", id).Updates(changes).Error
}
func (r *TaskRepository) ResetDaily() error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		today := time.Now().Format("2006-01-02")
		if err := tx.Where("assigned_date = ?", today).Delete(&model.PlayerTask{}).Error; err != nil {
			return err
		}
		// 全表重置每日任务标记：GORM 要求显式声明无 WHERE 意图
		return tx.Model(&model.Player{}).Where("1 = 1").Updates(map[string]any{"daily_task_date": ""}).Error
	})
}
