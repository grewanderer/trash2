package models

import "time"

type DeviceSecret struct {
	ID         uint   `gorm:"primaryKey"`
	DeviceID   uint   `gorm:"index"`
	KeyID      string `gorm:"index"`
	SecretHash []byte
	CreatedAt  time.Time
	RevokedAt  *time.Time
}
