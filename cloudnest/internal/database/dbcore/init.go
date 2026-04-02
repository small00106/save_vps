package dbcore

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/cloudnest/cloudnest/internal/database/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

func Init(dbType, dsn string) error {
	var initErr error
	once.Do(func() {
		cfg := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		}

		switch dbType {
		case "mysql":
			db, initErr = gorm.Open(mysql.Open(dsn), cfg)
		default: // sqlite
			// Ensure directory exists
			dir := filepath.Dir(dsn)
			if err := os.MkdirAll(dir, 0755); err != nil {
				initErr = err
				return
			}
			db, initErr = gorm.Open(sqlite.Open(dsn+"?_journal_mode=WAL"), cfg)
		}

		if initErr != nil {
			return
		}

		// Auto-migrate all models
		initErr = db.AutoMigrate(
			&models.Node{},
			&models.NodeMetric{},
			&models.NodeMetricCompact{},
			&models.File{},
			&models.FileReplica{},
			&models.User{},
			&models.Session{},
			&models.AlertRule{},
			&models.AlertChannel{},
			&models.PingTask{},
			&models.PingResult{},
			&models.CommandTask{},
			&models.AuditLog{},
		)
		if initErr != nil {
			return
		}

		log.Printf("Database initialized: %s", dbType)
	})
	return initErr
}

func DB() *gorm.DB {
	return db
}
