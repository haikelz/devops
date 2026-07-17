package config

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"time"

	"app/internal/env"
	"app/internal/models"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitGorm() *gorm.DB {
	cfg := env.GetEnv()
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormLogLevel(cfg.APP_DEBUG),
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)

	var dialector gorm.Dialector

	if cfg.TURSO_DATABASE_URL != "" && cfg.TURSO_AUTH_TOKEN != "" {
		dsn := cfg.TURSO_DATABASE_URL + "?authToken=" + cfg.TURSO_AUTH_TOKEN
		conn, err := sql.Open("libsql", dsn)
		if err != nil {
			log.Fatalf("failed to open turso connection: %v", err)
		}
		dialector = sqlite.New(sqlite.Config{Conn: conn})
	} else {
		dialector = sqlite.Open("ekel_backend.db")
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormLogger,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&models.GuestbookEntry{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func gormLogLevel(appDebug string) logger.LogLevel {
	if strings.EqualFold(appDebug, "true") {
		return logger.Info
	}

	return logger.Error
}
