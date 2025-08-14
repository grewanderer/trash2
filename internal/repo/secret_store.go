package repo

import (
	"context"
	"time"

	"wisp/internal/models"

	"gorm.io/gorm"
)

type SecretStore struct{ db *gorm.DB }

func NewSecretStore(db *gorm.DB) *SecretStore { return &SecretStore{db: db} }

func (s *SecretStore) Create(ctx context.Context, ds *models.DeviceSecret) error {
	return s.db.WithContext(ctx).Create(ds).Error
}
func (s *SecretStore) GetActive(ctx context.Context, deviceID uint, keyID string) (*models.DeviceSecret, error) {
	var res models.DeviceSecret
	err := s.db.WithContext(ctx).Where("device_id=? AND key_id=? AND revoked_at IS NULL", deviceID, keyID).First(&res).Error
	if err != nil {
		return nil, err
	}
	return &res, nil
}
func (s *SecretStore) RevokeAll(ctx context.Context, deviceID uint) error {
	return s.db.WithContext(ctx).Model(&models.DeviceSecret{}).Where("device_id=?", deviceID).
		Update("revoked_at", time.Now().UTC()).Error
}
