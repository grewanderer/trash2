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
	ID    uint   `gorm:"primaryKey"`
	UUID  string `gorm:"type:char(36);uniqueIndex;not null"` // ← было type:uuid
	Name  string `gorm:"type:text"`
	OrgID *uint  `gorm:"index"`
	Model string `gorm:"type:text"`
	MAC   string `gorm:"type:text"`

	Fingerprint   string         `gorm:"type:text"`
	Tags          datatypes.JSON `gorm:"type:json"`
	Status        DeviceStatus   `gorm:"type:text;default:'unknown'"`
	LastSeenAt    *time.Time
	ConfigArchive []byte // ← убрали gorm:"type:bytea" (MySQL не знает bytea)
	Key           string `gorm:"column:device_key;uniqueIndex"` // НЕ "key"

	ConfigVersion      int            `gorm:"default:0"`
	ConfigChecksum     string         `gorm:"type:text"`
	DesiredConfig      datatypes.JSON `gorm:"type:json"`
	RenderedConfig     datatypes.JSON `gorm:"type:json"`
	ConfigUpdatedAt    *time.Time
	LastReportedStatus string `gorm:"type:text"`
	LastAppliedAt      *time.Time
	LastAppliedSum     string `gorm:"type:char(64)"` // sha256 sum, совпадает с выдаваемым checksum при download

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
