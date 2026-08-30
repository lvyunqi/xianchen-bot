package storage

import (
	"gorm.io/gorm"
	"time"
	"xianlv/internal/model"
)

type LogRepository struct{ db *gorm.DB }

func NewLogRepository(db *gorm.DB) *LogRepository { return &LogRepository{db: db} }
func (r *LogRepository) Game(level, kind string, playerID uint, message, metadata string) error {
	return r.db.Create(&model.GameLog{Level: level, Type: kind, PlayerID: playerID, Message: message, MetadataJSON: metadata}).Error
}
func (r *LogRepository) Operation(row *model.OperationLog) error { return r.db.Create(row).Error }
func (r *LogRepository) List(kind, level string, start, end time.Time, limit int) (rows []model.GameLog, err error) {
	db := r.db
	if kind != "" {
		db = db.Where("type = ?", kind)
	}
	if level != "" {
		db = db.Where("level = ?", level)
	}
	if !start.IsZero() {
		db = db.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		db = db.Where("created_at <= ?", end)
	}
	err = db.Order("id DESC").Limit(limit).Find(&rows).Error
	return
}
