package server

import (
	"context"
	"fmt"
	"time"

	"wisp/internal/owctrl"
	"wisp/internal/repo"
)

type storeAdapter struct{ ds *repo.DeviceStore }

func newStoreAdapter(ds *repo.DeviceStore) owctrl.Store { return &storeAdapter{ds: ds} }

func (a *storeAdapter) Adopt(ctxCtx interface{ Done() <-chan struct{} }, in owctrl.AdoptRequest) (*owctrl.DeviceDTO, error) {
	ctx, _ := ctxCtx.(context.Context)
	dev, err := a.ds.Adopt(ctx, repo.AdoptInput{
		UUID:        in.UUID,
		Fingerprint: in.Fingerprint,
		Metadata:    in.Metadata,
	})
	if err != nil {
		return nil, err
	}
	return &owctrl.DeviceDTO{ID: toStringID(dev.ID), UUID: dev.UUID, Name: dev.Name}, nil
}

func (a *storeAdapter) GetConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string) ([]byte, int, string, error) {
	ctx, _ := ctxCtx.(context.Context)
	return a.ds.GetConfigNoKey(ctx, uuid) // ← новое имя
}

func (a *storeAdapter) AckConfig(ctxCtx interface{ Done() <-chan struct{} }, uuid string, version int, checksum, status string, appliedAt time.Time) error {
	ctx, _ := ctxCtx.(context.Context)
	return a.ds.AckConfigOW(ctx, uuid, version, checksum, status, appliedAt) // ← новое имя
}

func toStringID(id uint) string { return fmt.Sprintf("%d", id) }
