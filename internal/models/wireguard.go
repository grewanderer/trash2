package models

type WireGuardPeer struct {
	ID           uint `gorm:"primaryKey"`
	DeviceID     uint `gorm:"index"`
	PrivateKey   string
	PublicKey    string
	PresharedKey string
	AddressCIDR  string // "10.10.0.X/32"
	ServerPub    string
	Endpoint     string // "host:port"
	AllowedIPs   string // CSV
	Keepalive    int
}
