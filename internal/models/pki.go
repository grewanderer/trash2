package models

import "time"

type CA struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex"`
	CertPEM   []byte
	KeyPEM    []byte
	NotBefore time.Time
	NotAfter  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Certificate struct {
	ID        uint  `gorm:"primaryKey"`
	CAID      uint  `gorm:"index"`
	DeviceID  *uint `gorm:"index"`
	CN        string
	CertPEM   []byte
	KeyPEM    []byte
	NotBefore time.Time
	NotAfter  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
