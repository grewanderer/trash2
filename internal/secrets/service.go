package secrets

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"wisp/internal/models"
	"wisp/internal/repo"

	"golang.org/x/crypto/argon2"
)

type Service struct{ Store *repo.SecretStore }

func New(store *repo.SecretStore) *Service { return &Service{Store: store} }

func (s *Service) Issue(ctx context.Context, deviceID uint) (keyID string, secret []byte, err error) {
	var raw [32]byte
	_, _ = rand.Read(raw[:])
	secret = raw[:]
	keyID = hex.EncodeToString(raw[:6]) // короткий id
	hash := argon2.IDKey(secret, []byte("ow-hmac"), 1, 64*1024, 1, 32)
	err = s.Store.Create(ctx, &models.DeviceSecret{DeviceID: deviceID, KeyID: keyID, SecretHash: hash})
	return
}
func (s *Service) Verify(secretHash []byte, candidate []byte) bool {
	h := argon2.IDKey(candidate, []byte("ow-hmac"), 1, 64*1024, 1, 32)
	if len(h) != len(secretHash) {
		return false
	}
	ok := true
	for i := range h {
		ok = ok && (h[i] == secretHash[i])
	}
	return ok
}
