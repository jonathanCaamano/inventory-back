package database

import (
	"log/slog"
	"time"

	"github.com/jonathanCaamano/inventory-back/internal/config"
	"github.com/jonathanCaamano/inventory-back/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	maxRetries    = 5
	retryInterval = 3 * time.Second
)

func Connect(cfg *config.Config) (*gorm.DB, error) {
	logLevel := logger.Error
	if !cfg.IsProduction() {
		logLevel = logger.Warn
	}

	var db *gorm.DB
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		db, err = gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
			Logger: logger.Default.LogMode(logLevel),
		})
		if err == nil {
			sqlDB, pingErr := db.DB()
			if pingErr == nil {
				pingErr = sqlDB.Ping()
			}
			if pingErr == nil {
				break
			}
			err = pingErr
		}
		if attempt < maxRetries {
			slog.Warn("database connection failed, retrying",
				slog.Int("attempt", attempt),
				slog.Duration("next_retry", retryInterval),
				slog.String("error", err.Error()),
			)
			time.Sleep(retryInterval)
		}
	}
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	slog.Info("database connected")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Product{},
		&models.Contact{},
		&models.ProductImage{},
		&models.RefreshToken{},
	); err != nil {
		return err
	}
	return runCompatMigrations(db)
}

// runCompatMigrations applies one-off SQL changes that AutoMigrate cannot
// handle (e.g. relaxing constraints on existing columns).
func runCompatMigrations(db *gorm.DB) error {
	stmts := []string{
		// contacts.subdato was previously NOT NULL; the field is no longer
		// part of the API so new inserts omit it — make it nullable.
		`ALTER TABLE contacts ALTER COLUMN subdato DROP NOT NULL`,
		// Give existing products without a status the default value.
		`UPDATE products SET status = 'en_progreso' WHERE status = '' OR status IS NULL`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			// Log but don't fail — statement may already be applied.
			slog.Warn("compat migration skipped", slog.String("stmt", stmt), slog.String("error", err.Error()))
		}
	}
	return nil
}
