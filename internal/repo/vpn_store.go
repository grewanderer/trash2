package repo

import (
	"context"
	"errors"

	"wisp/internal/models"

	"gorm.io/gorm"
)

func (s *DeviceStore) EnsureWGPeer(ctx context.Context, deviceID uint, newPeer func() (*models.WireGuardPeer, error)) (*models.WireGuardPeer, error) {
	var p models.WireGuardPeer
	err := s.db.WithContext(ctx).Where("device_id=?", deviceID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		np, err := newPeer()
		if err != nil {
			return nil, err
		}
		np.DeviceID = deviceID
		if err := s.db.WithContext(ctx).Create(np).Error; err != nil {
			return nil, err
		}
		return np, nil
	}
	return &p, err
}
