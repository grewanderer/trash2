package repo

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"wisp/internal/models"
)

type TemplateStore struct{ db *gorm.DB }

func NewTemplateStore(db *gorm.DB) *TemplateStore { return &TemplateStore{db: db} }

// Пока просто отдаём все шаблоны по возрастанию priority (org/site/role можно добавить позже).
func (s *TemplateStore) ListForDevice(ctx context.Context, deviceID uint) ([]models.ConfigTemplate, error) {
	var tpls []models.ConfigTemplate
	if err := s.db.WithContext(ctx).
		Order("priority asc, id asc").
		Find(&tpls).Error; err != nil {
		return nil, err
	}
	return tpls, nil
}

// Базовые переменные для ApplyVars (расширите при необходимости).
func (s *TemplateStore) VarsForDevice(ctx context.Context, dev *models.Device) (map[string]any, error) {
	return map[string]any{
		"device_uuid": dev.UUID,
		"device_name": dev.Name,
		"model":       dev.Model,
		"mac":         dev.MAC,
	}, nil
}

// DecodeNetJSON — хелпер для распаковки JSON поля шаблона в map[string]any.
func DecodeNetJSON(t models.ConfigTemplate) (map[string]any, error) {
	if len(t.NetJSON) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(t.NetJSON, &m); err != nil {
		return nil, err
	}
	return m, nil
}
