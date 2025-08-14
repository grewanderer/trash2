package models

import (
	"time"

	"gorm.io/datatypes"
)

type ConfigTemplate struct {
	ID                   uint           `gorm:"primaryKey"`
	OrgID                *uint          `gorm:"index"`
	Name                 string         `gorm:"uniqueIndex:tpl_scope"`
	Priority             int            `gorm:"default:100"`
	NetJSON              datatypes.JSON `gorm:"type:jsonb"`
	VarsSchema           datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt, UpdatedAt time.Time
}
