package secrets

import (
	"net/http"
	"sync"
	"time"

	"wisp/internal/models"

	"gorm.io/gorm"
)

type DBKeyProvider struct {
	db    *gorm.DB
	nonce sync.Map // keyID -> map[nonce]ts  (на старте product можно так; затем Redis)
	ttl   time.Duration
}

func NewDBKeyProvider(db *gorm.DB, ttl time.Duration) *DBKeyProvider {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &DBKeyProvider{db: db, ttl: ttl}
}

type KeyInfo struct {
	KeyID  string
	Secret []byte
}

func (p *DBKeyProvider) Lookup(_ *http.Request, deviceUUID, keyID string) (*KeyInfo, error) {
	var dev models.Device
	if err := p.db.Where("uuid=?", deviceUUID).First(&dev).Error; err != nil {
		return nil, err
	}
	var ds models.DeviceSecret
	if err := p.db.Where("device_id=? AND key_id=? AND revoked_at IS NULL", dev.ID, keyID).First(&ds).Error; err != nil {
		return nil, err
	}
	// В product лучше хранить секрет шифрованно; здесь — не восстанавливаем (hash only).
	// Для HMAC нужна «сырая» секреt-строка — её отдаём при Issue и кэшируем у агента.
	// Здесь вернём nil -> используйте MemKeyProvider для онлайн-проверки, либо храните зашифрованный секрет.
	return &KeyInfo{KeyID: keyID, Secret: nil}, nil
}

func (p *DBKeyProvider) SeenNonce(keyID, nonce string, at time.Time) (bool, error) {
	keyMapAny, _ := p.nonce.LoadOrStore(keyID, &sync.Map{})
	keyMap := keyMapAny.(*sync.Map)
	if _, ok := keyMap.Load(nonce); ok {
		return true, nil
	}
	// gc по ttl
	keyMap.Range(func(k, v any) bool {
		if ts, ok := v.(time.Time); ok && ts.Before(time.Now().Add(-p.ttl)) {
			keyMap.Delete(k)
		}
		return true
	})
	keyMap.Store(nonce, at)
	return false, nil
}
