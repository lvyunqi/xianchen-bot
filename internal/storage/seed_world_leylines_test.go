package storage

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"xianlv/internal/config"
	"xianlv/internal/model"
)

func TestMigrateFullLegacyWorldLeylineCatalogWithoutNameCollision(t *testing.T) {
	cfg := config.Runtime(t.TempDir())
	cfg.Database.DSN = filepath.Join(t.TempDir(), "full-legacy-leylines.db")
	store, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.WorldLeyline{}).Error; err != nil {
		t.Fatal(err)
	}
	legacy := make([]model.WorldLeyline, 0, 1000)
	for index := 1; index <= 1000; index++ {
		legacy = append(legacy, legacyWorldLeylineProfile(index))
	}
	if err := store.DB.CreateInBatches(&legacy, 100).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.migrateWorldLeylineCatalog(); err != nil {
		t.Fatalf("migrate full legacy catalogue: %v", err)
	}
	for _, element := range worldLeylineElements {
		var count int64
		if err := store.DB.Model(&model.WorldLeyline{}).Where("element = ? AND required_root_element = ?", element, element).Count(&count).Error; err != nil || count != 100 {
			t.Fatalf("element %s count=%d err=%v", element, count, err)
		}
	}
	var obsolete int64
	if err := store.DB.Model(&model.WorldLeyline{}).Where("element IN ? OR required_root_element IN ?", []string{"风雷", "太阴", "太阳", "星辰"}, []string{"风雷", "太阴", "太阳", "星辰"}).Count(&obsolete).Error; err != nil || obsolete != 0 {
		t.Fatalf("obsolete origins=%d err=%v", obsolete, err)
	}
	var opening []model.WorldLeyline
	if err := store.DB.Where("minimum_realm_sequence = ? AND minimum_realm_level = ? AND location_name = ?", 1, 1, "青云山脚").Order("sort_order").Find(&opening).Error; err != nil || len(opening) != 10 {
		t.Fatalf("opening leyline count=%d err=%v", len(opening), err)
	}
}
