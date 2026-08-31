package storage

import (
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func sqliteDialector(path string) gorm.Dialector {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	return sqlite.Open(dsn)
}
