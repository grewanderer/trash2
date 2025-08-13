package db

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open подключает БД по driver/dsn.
// Поддержка: "mysql" | "postgres" | "" (нет БД, in-memory режим).
func Open(driver, dsn string) (*gorm.DB, error) {
	switch driver {
	case "":
		return nil, nil
	case "mysql":
		// Пример DSN:
		// user:pass@tcp(127.0.0.1:3306)/openwisp?parseTime=true&charset=utf8mb4&loc=Local
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		// Пример DSN:
		// postgres://user:pass@localhost:5432/openwisp?sslmode=disable
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}
