package repo

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wisp/internal/models"
)

var (
	ErrNotFound     = errors.New("device not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrBadSecret    = errors.New("bad shared secret")
)

type DeviceStore struct{ db *gorm.DB }

func NewDeviceStore(db *gorm.DB) *DeviceStore { return &DeviceStore{db: db} }

type RegisterInput struct {
	SharedSecret   string
	ExpectedSecret string
	Name           string
	MAC            string
	Model          string // ← было Backend
	KeyOptional    string
	ConsistentKey  bool
}
type RegisterResult struct {
	UUID  string
	Key   string
	Name  string
	IsNew bool
}

// -------- Register (для /controller/register/) --------
func (s *DeviceStore) Register(ctx context.Context, in RegisterInput) (*RegisterResult, error) {
	// 1) проверяем shared secret
	if strings.TrimSpace(in.ExpectedSecret) != "" && in.SharedSecret != in.ExpectedSecret {
		return nil, ErrBadSecret
	}

	// 2) вычисляем/берём key
	key := strings.TrimSpace(in.KeyOptional)
	if key == "" {
		if in.ConsistentKey && in.MAC != "" && in.ExpectedSecret != "" {
			h := md5.Sum([]byte(strings.ToLower(in.MAC) + in.ExpectedSecret))
			key = hex.EncodeToString(h[:]) // совместимо с openwisp-consistent-key
		} else {
			// случайный 16-байтовый hex (32 символа)
			u := uuid.New()
			h := md5.Sum([]byte(u.String()))
			key = hex.EncodeToString(h[:])
		}
	}

	// 3) если устройство уже есть по key — вернём его (STRUCT Where: `devices`.`key`)
	var existing models.Device
	err := s.db.WithContext(ctx).Where(&models.Device{Key: key}).First(&existing).Error
	if err == nil {
		// обновим метаданные "по дороге"
		changed := false
		if in.Name != "" && existing.Name != in.Name {
			existing.Name = in.Name
			changed = true
		}
		if in.Model != "" && existing.Model != in.Model {
			existing.Model = in.Model
			changed = true
		}
		if in.MAC != "" && existing.MAC != in.MAC {
			existing.MAC = in.MAC
			changed = true
		}
		if changed {
			_ = s.db.WithContext(ctx).Save(&existing).Error
		}
		return &RegisterResult{
			UUID:  existing.UUID,
			Key:   existing.Key,
			Name:  existing.Name,
			IsNew: false,
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 4) создаём новое устройство
	d := models.Device{
		UUID:   uuid.NewString(),
		Name:   in.Name,
		Model:  in.Model,
		MAC:    in.MAC,
		Key:    key,
		Status: models.DeviceStatusUnknown,
	}
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now

	if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
		return nil, err
	}
	return &RegisterResult{
		UUID:  d.UUID,
		Key:   d.Key,
		Name:  d.Name,
		IsNew: true,
	}, nil
}

// -------- Агентские методы (uuid+key) уже есть --------

func (s *DeviceStore) ValidateKey(ctx context.Context, uuid, key string) (*models.Device, error) {
	var d models.Device
	err := s.db.WithContext(ctx).Where(&models.Device{UUID: uuid, Key: key}).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUnauthorized
	}
	return &d, err
}

func (s *DeviceStore) GetChecksum(ctx context.Context, uuid, key string) (string, error) {
	d, err := s.ValidateKey(ctx, uuid, key)
	if err != nil {
		return "", err
	}
	if len(d.ConfigArchive) == 0 || d.ConfigChecksum == "" {
		return "", ErrNotFound
	}
	return d.ConfigChecksum, nil
}

func (s *DeviceStore) GetConfig(ctx context.Context, uuid, key string) ([]byte, string, error) {
	d, err := s.ValidateKey(ctx, uuid, key)
	if err != nil {
		return nil, "", err
	}
	if len(d.ConfigArchive) == 0 {
		return nil, "", ErrNotFound
	}
	return d.ConfigArchive, d.ConfigChecksum, nil
}

func (s *DeviceStore) PutConfigTar(ctx context.Context, uuid string, tarGz []byte, version int) error {
	var d models.Device
	if err := s.db.WithContext(ctx).Where("uuid=?", uuid).First(&d).Error; err != nil {
		return err
	}
	sum := sha256.Sum256(tarGz)
	d.ConfigArchive = tarGz
	d.ConfigChecksum = hex.EncodeToString(sum[:])
	d.ConfigVersion = version
	now := time.Now().UTC()
	d.ConfigUpdatedAt = &now
	return s.db.WithContext(ctx).Save(&d).Error
}

func (s *DeviceStore) ReportStatus(ctx context.Context, uuid, key, status string) error {
	d, err := s.ValidateKey(ctx, uuid, key)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	d.LastSeenAt = &now
	d.LastReportedStatus = status

	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "online":
		d.Status = models.DeviceStatusOnline
	case "ok", "applied", "success":
		d.Status = models.DeviceStatusOnline
	case "error", "failed", "offline":
		d.Status = models.DeviceStatusOffline
	default:
		d.Status = models.DeviceStatusOnline
	}
	return s.db.WithContext(ctx).Save(d).Error
}

func (s *DeviceStore) ReportApplied(ctx context.Context, uuid, key, localSum string) error {
	d, err := s.ValidateKey(ctx, uuid, key)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if strings.TrimSpace(localSum) == "" {
		localSum = d.ConfigChecksum
	}
	d.LastAppliedAt = &now
	d.LastAppliedSum = localSum
	d.Status = models.DeviceStatusOnline
	return s.db.WithContext(ctx).Save(d).Error
}

func (s *DeviceStore) MarkSeen(ctx context.Context, uuid string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&models.Device{}).
		Where("uuid = ?", uuid).
		Updates(map[string]any{
			"last_seen_at": now,
			"status":       models.DeviceStatusOnline,
		}).Error
}

// -------- Методы под owctrl-адаптер (без key) --------

type AdoptInput struct {
	UUID        string
	Fingerprint string
	Metadata    map[string]any
}

func (s *DeviceStore) Adopt(ctx context.Context, in AdoptInput) (*models.Device, error) {
	var d models.Device
	tx := s.db.WithContext(ctx)
	err := tx.Where("uuid=?", in.UUID).First(&d).Error
	now := time.Now().UTC()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		d = models.Device{
			UUID:        in.UUID,
			Fingerprint: in.Fingerprint,
			Status:      models.DeviceStatusOnline,
			LastSeenAt:  &now,
		}
		if in.Metadata != nil {
			if name, ok := in.Metadata["name"].(string); ok {
				d.Name = name
			}
			if mac, ok := in.Metadata["mac"].(string); ok {
				d.MAC = mac
			}
			if mdl, ok := in.Metadata["model"].(string); ok {
				d.Model = mdl
			}
		}
		if err := tx.Create(&d).Error; err != nil {
			return nil, err
		}
		return &d, nil
	}
	if err != nil {
		return nil, err
	}

	d.Fingerprint = in.Fingerprint
	d.Status = models.DeviceStatusOnline
	d.LastSeenAt = &now
	if in.Metadata != nil {
		if name, ok := in.Metadata["name"].(string); ok && name != "" {
			d.Name = name
		}
		if mac, ok := in.Metadata["mac"].(string); ok && mac != "" {
			d.MAC = mac
		}
		if mdl, ok := in.Metadata["model"].(string); ok && mdl != "" {
			d.Model = mdl
		}
	}
	if err := tx.Save(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *DeviceStore) GetByUUID(ctx context.Context, uuid string) (*models.Device, error) {
	var d models.Device
	err := s.db.WithContext(ctx).Where("uuid=?", uuid).First(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &d, err
}

func (s *DeviceStore) GetConfigNoKey(ctx context.Context, uuid string) ([]byte, int, string, error) {
	d, err := s.GetByUUID(ctx, uuid)
	if err != nil {
		return nil, 0, "", err
	}
	if d == nil {
		return nil, 0, "", gorm.ErrRecordNotFound
	}

	// рендер: берём RenderedConfig → DesiredConfig → пустой NetJSON
	cfg := d.RenderedConfig
	if len(cfg) == 0 {
		cfg = d.DesiredConfig
	}
	if len(cfg) == 0 {
		cfg, _ = json.Marshal(map[string]any{
			"type": "DeviceConfiguration",
		})
	}
	h := sha256.Sum256(cfg)
	sum := "sha256:" + hex.EncodeToString(h[:])
	ver := d.ConfigVersion
	if ver == 0 {
		ver = 1
	}
	return cfg, ver, sum, nil
}

func (s *DeviceStore) AckConfigOW(ctx context.Context, uuid string, version int, checksum, status string, appliedAt time.Time) error {
	d, err := s.GetByUUID(ctx, uuid)
	if err != nil {
		return err
	}
	if d == nil {
		return gorm.ErrRecordNotFound
	}
	d.ConfigVersion = version
	d.ConfigChecksum = checksum
	d.ConfigUpdatedAt = &appliedAt
	d.Status = models.DeviceStatusOnline
	return s.db.WithContext(ctx).Save(d).Error
}
