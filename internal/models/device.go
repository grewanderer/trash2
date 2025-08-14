package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DeviceStatus string

const (
	DeviceStatusUnknown DeviceStatus = "unknown"
	DeviceStatusOnline  DeviceStatus = "online"
	DeviceStatusOffline DeviceStatus = "offline"
)

type Device struct {
	ID              uint           `gorm:"primaryKey"`
	UUID            string         `gorm:"type:uuid;uniqueIndex;not null"`
	Name            string         `gorm:"type:text"`
	OrgID           *uint          `gorm:"index"`
	Model           string         `gorm:"type:text"`
	MAC             string         `gorm:"type:text"`
	Fingerprint     string         `gorm:"type:text"`
	Tags            datatypes.JSON `gorm:"type:jsonb"`
	Status          DeviceStatus   `gorm:"type:text;default:'unknown'"`
	LastSeenAt      *time.Time
	ConfigArchive   []byte         `gorm:"type:bytea"`
	Key             string         `gorm:"type:char(32);index"`
	ConfigVersion   int            `gorm:"default:0"`
	ConfigChecksum  string         `gorm:"type:text"`
	DesiredConfig   datatypes.JSON `gorm:"type:jsonb"`
	RenderedConfig  datatypes.JSON `gorm:"type:jsonb"`
	ConfigUpdatedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
