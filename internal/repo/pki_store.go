package repo

import (
	"context"
	"wisp/internal/models"

	"gorm.io/gorm"
)

type PKIStore struct{ db *gorm.DB }

func NewPKIStore(db *gorm.DB) *PKIStore { return &PKIStore{db: db} }

func (s *PKIStore) GetOrCreateCA(ctx context.Context, name string, create func() (*models.CA, error)) (*models.CA, error) {
	var ca models.CA
	if err := s.db.WithContext(ctx).Where("name=?", name).First(&ca).Error; err == nil {
		return &ca, nil
	}
	newCA, err := create()
	if err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Create(newCA).Error; err != nil {
		return nil, err
	}
	return newCA, nil
}

func (s *PKIStore) SaveCert(ctx context.Context, c *models.Certificate) error {
	return s.db.WithContext(ctx).Create(c).Error
}
