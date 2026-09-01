package storage

import (
	"errors"

	"gorm.io/gorm"
)

// firstOrCreateCodeName keeps legacy catalogues upgradeable when an older row
// already owns the new display name under a numeric or otherwise obsolete code.
// It preserves the existing row (and therefore its foreign-key ID and operator
// edits), adopts the canonical code, and only creates a row when neither unique
// key exists.
func (s *Store) firstOrCreateCodeName(row any, code, name string) error {
	type identity struct {
		ID uint
	}
	var existing identity
	if err := s.DB.Model(row).Select("id").Where("code = ?", code).Take(&existing).Error; err == nil {
		return s.DB.First(row, existing.ID).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := s.DB.Model(row).Select("id").Where("name = ?", name).Take(&existing).Error; err == nil {
		if err := s.DB.Model(row).Where("id = ?", existing.ID).Update("code", code).Error; err != nil {
			return err
		}
		return s.DB.First(row, existing.ID).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.DB.Create(row).Error
}
