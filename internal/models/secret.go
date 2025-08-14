package models

import (
	"time"

	"gorm.io/gorm"
)

type DeviceSecret struct {
	ID         uint           `gorm:"primaryKey"`
	DeviceID   uint           `gorm:"index;not null;uniqueIndex:uniq_dev_key,priority:1"`
	KeyID      string         `gorm:"size:16;not null;uniqueIndex:uniq_dev_key,priority:2"` // короткий человекочитаемый id
	SecretHash []byte         `gorm:"type:varbinary(64);not null"`                          // храните только хэш секрета
	CreatedAt  time.Time      `gorm:"not null"`
	RevokedAt  *time.Time     `gorm:"index"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}
