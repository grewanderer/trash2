package owctrl

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type memDevice struct {
	ID       string
	UUID     string
	Name     string
	Version  int
	Checksum string
	Config   json.RawMessage
}

type memStore struct {
	mu      sync.RWMutex
	devices map[string]*memDevice
}

func newInMemoryStore() Store {
	return &memStore{devices: make(map[string]*memDevice)}
}

func (m *memStore) Adopt(ctxCtx interface{ Done() <-chan struct{} }, in AdoptRequest) (*DeviceDTO, error) {
	ctx, _ := ctxCtx.(context.Context)
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.devices[in.UUID]; ok {
		return &DeviceDTO{ID: d.ID, UUID: d.UUID, Name: d.Name}, nil
	}
	id := fmt.Sprintf("mem-%d", len(m.devices)+1)
	m.devices[in.UUID] = &memDevice{
		ID:      id,
		UUID:    in.UUID,
		Version: 1,
		Config:  json.RawMessage(`{"type":"DeviceConfiguration","network":{}}`),
	}
	return &DeviceDTO{ID: id, UUID: in.UUID}, nil
}

func (m *memStore) GetConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string) ([]byte, int, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.devices[uuid]
	if !ok {
		return nil, 0, "", fmt.Errorf("device not found")
	}
	sum := checksum(d.Config)
	return d.Config, d.Version, sum, nil
}

func (m *memStore) AckConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string, version int, checksum, status string, appliedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.devices[uuid]
	if !ok {
		return fmt.Errorf("device not found")
	}
	d.Version = version
	d.Checksum = checksum
	_ = status
	_ = appliedAt
	return nil
}
