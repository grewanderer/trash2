package models

import (
	"time"

	"gorm.io/gorm"
)

type Device struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UUID      string `gorm:"uniqueIndex;size:64;not null" json:"uuid"`
	DeviceKey string `gorm:"index;size:255;not null" json:"device_key"`
	Name      string `gorm:"size:255" json:"name"`
	Backend   string `gorm:"size:255" json:"backend"`
	MAC       string `gorm:"size:64"  json:"mac"`
	Status    string `gorm:"size:64"  json:"status"`
}
