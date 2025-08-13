package server

import (
	"wisp/internal/owctrl"
	"wisp/internal/repo"
)

/*━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━*
|  ADAPTER: repo.DeviceStore -> owctrl.Store                    |
*━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━*/

type storeAdapter struct{ ds *repo.DeviceStore }

func newStoreAdapter(ds *repo.DeviceStore) *storeAdapter { return &storeAdapter{ds: ds} }

var _ owctrl.Store = (*storeAdapter)(nil)

func (s *storeAdapter) UpsertByKey(key string, d owctrl.DeviceFields) (owctrl.DeviceFields, bool) {
	df, isNew := s.ds.UpsertByKey(key, repo.DeviceFields{
		UUID:    d.UUID,
		Key:     d.Key,
		Name:    d.Name,
		Backend: d.Backend,
		MAC:     d.MAC,
		Status:  d.Status,
	})
	return owctrl.DeviceFields{
		UUID:      df.UUID,
		Key:       key, // ключ подтверждаем внешним аргументом
		Name:      df.Name,
		Backend:   df.Backend,
		MAC:       df.MAC,
		Status:    df.Status,
		UpdatedAt: df.UpdatedAt,
	}, isNew
}

func (s *storeAdapter) FindByUUID(id string) (owctrl.DeviceFields, bool) {
	df, ok := s.ds.FindByUUID(id)
	if !ok {
		return owctrl.DeviceFields{}, false
	}
	return owctrl.DeviceFields{
		UUID:      df.UUID,
		Key:       df.Key,
		Name:      df.Name,
		Backend:   df.Backend,
		MAC:       df.MAC,
		Status:    df.Status,
		UpdatedAt: df.UpdatedAt,
	}, true
}

func (s *storeAdapter) UpdateStatus(id, status string) error {
	return s.ds.UpdateStatus(id, status)
}
