// internal/owctrl/hmac_mem_provider.go
package owctrl

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

type MemKeyProvider struct {
	mu     sync.Mutex
	keys   map[string]KeyInfo
	nonces map[string]map[string]time.Time
	ttl    time.Duration
}

func NewMemKeyProvider(ttl time.Duration) *MemKeyProvider {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &MemKeyProvider{
		keys:   make(map[string]KeyInfo),
		nonces: make(map[string]map[string]time.Time),
		ttl:    ttl,
	}
}

func (m *MemKeyProvider) Put(deviceUUID, keyID string, secret []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys[deviceUUID] = KeyInfo{KeyID: keyID, Secret: secret}
}

func (m *MemKeyProvider) Lookup(_ *http.Request, deviceUUID, keyID string) (*KeyInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ki, ok := m.keys[deviceUUID]
	if !ok || ki.KeyID != keyID {
		return nil, errors.New("not found")
	}
	return &ki, nil
}

func (m *MemKeyProvider) SeenNonce(keyID, nonce string, at time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nonces[keyID] == nil {
		m.nonces[keyID] = make(map[string]time.Time)
	}
	// GC старых nonce
	cut := time.Now().Add(-m.ttl)
	for n, ts := range m.nonces[keyID] {
		if ts.Before(cut) {
			delete(m.nonces[keyID], n)
		}
	}
	if _, ok := m.nonces[keyID][nonce]; ok {
		return true, nil
	}
	m.nonces[keyID][nonce] = at
	return false, nil
}
