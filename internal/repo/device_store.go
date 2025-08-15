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
	"gorm.io/gorm/clause"

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
	// 1) shared secret (если в конфиге задан)
	if sec := strings.TrimSpace(in.ExpectedSecret); sec != "" && in.SharedSecret != sec {
		return nil, ErrBadSecret
	}

	// нормализуем MAC (для consistent key)
	mac := strings.ToLower(strings.TrimSpace(in.MAC))

	// 2) выбрать/сгенерировать key
	key := strings.TrimSpace(in.KeyOptional)
	if key == "" {
		if in.ConsistentKey && mac != "" && in.ExpectedSecret != "" {
			sum := md5.Sum([]byte(mac + in.ExpectedSecret)) // совместимо с openwisp-consistent-key
			key = hex.EncodeToString(sum[:])
		} else {
			sum := md5.Sum([]byte(uuid.NewString()))
			key = hex.EncodeToString(sum[:]) // 32 hex
		}
	}

	now := time.Now().UTC()

	// 3) быстрый путь: устройство уже существует по key
	var d models.Device
	if err := s.db.WithContext(ctx).
		Where("device_key = ?", key). // ВАЖНО: колонка, а не reserved word KEY
		First(&d).Error; err == nil {

		updates := map[string]any{}
		if n := strings.TrimSpace(in.Name); n != "" && d.Name != n {
			updates["name"] = n
		}
		if m := strings.TrimSpace(in.Model); m != "" && d.Model != m {
			updates["model"] = m
		}
		if mac != "" && d.MAC != mac {
			updates["mac"] = mac
		}
		if len(updates) > 0 {
			updates["updated_at"] = now
			_ = s.db.WithContext(ctx).Model(&d).Updates(updates).Error
		}
		return &RegisterResult{UUID: d.UUID, Key: d.Key, Name: d.Name, IsNew: false}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 4) создать новое устройство (идемпотентно на случай гонки)
	d = models.Device{
		UUID:      uuid.NewString(),
		Name:      strings.TrimSpace(in.Name),
		Model:     strings.TrimSpace(in.Model),
		MAC:       mac,
		Key:       key, // поле модели должно быть с тегом: gorm:"column:device_key;uniqueIndex"
		Status:    models.DeviceStatusUnknown,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// ON CONFLICT (device_key) DO NOTHING → если уже кто-то успел создать — просто перечитаем
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_key"}},
			DoNothing: true,
		}).
		Create(&d).Error; err != nil {
		return nil, err
	}

	// Если был конфликт — загрузим созданную ранее запись и вернём её
	if d.ID == 0 {
		if err := s.db.WithContext(ctx).Where("device_key = ?", key).First(&d).Error; err != nil {
			return nil, err
		}
		return &RegisterResult{UUID: d.UUID, Key: d.Key, Name: d.Name, IsNew: false}, nil
	}

	return &RegisterResult{UUID: d.UUID, Key: d.Key, Name: d.Name, IsNew: true}, nil
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
