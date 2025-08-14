// internal/owctrl/auth_hmac.go
package owctrl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"
)

type KeyInfo struct {
	KeyID  string
	Secret []byte
	// опционально: срок годности, статус и т.п.
}

type KeyProvider interface {
	// По устройству (uuid) и/или keyId вернуть секрет.
	Lookup(r *http.Request, deviceUUID, keyID string) (*KeyInfo, error)
	// Проверка/фиксация nonce, чтобы избежать повторов.
	SeenNonce(keyID, nonce string, at time.Time) (already bool, err error)
}

func HMACAuth(p KeyProvider, maxSkew time.Duration) func(http.Handler) http.Handler {
	if maxSkew <= 0 {
		maxSkew = 5 * time.Minute
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dev := r.Header.Get("X-OW-Device")
			date := r.Header.Get("X-OW-Date")
			nonce := r.Header.Get("X-OW-Nonce")
			bodyHash := r.Header.Get("X-OW-Body-SHA256")
			auth := r.Header.Get("Authorization")

			fail := func(code int, msg string) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
			}

			if dev == "" || date == "" || nonce == "" || auth == "" {
				fail(http.StatusUnauthorized, "missing auth headers")
				return
			}
			// Парсим Authorization: "OW1-HMAC-SHA256 keyId:signatureHex"
			const scheme = "OW1-HMAC-SHA256 "
			if !strings.HasPrefix(auth, scheme) {
				fail(http.StatusUnauthorized, "bad scheme")
				return
			}
			authv := strings.TrimPrefix(auth, scheme)
			colon := strings.IndexByte(authv, ':')
			if colon <= 0 {
				fail(http.StatusUnauthorized, "bad authorization format")
				return
			}
			keyID := authv[:colon]
			sigHex := authv[colon+1:]
			if len(sigHex) < 20 {
				fail(http.StatusUnauthorized, "bad signature")
				return
			}

			// Проверка времени
			ts, err := time.Parse(time.RFC3339, date)
			if err != nil {
				fail(http.StatusUnauthorized, "bad date")
				return
			}
			now := time.Now().UTC()
			if ts.After(now.Add(maxSkew)) || ts.Before(now.Add(-maxSkew)) {
				fail(http.StatusUnauthorized, "clock skew")
				return
			}

			// Читаем тело и восстанавливаем его
			rawBody, err := io.ReadAll(r.Body)
			if err != nil {
				fail(http.StatusBadRequest, "read body")
				return
			}
			_ = r.Body.Close()
			r.Body = io.NopCloser(strings.NewReader(string(rawBody)))

			h := sha256.Sum256(rawBody)
			calculatedBody := hex.EncodeToString(h[:])
			if bodyHash != "" && !strings.EqualFold(bodyHash, calculatedBody) {
				fail(http.StatusUnauthorized, "body hash mismatch")
				return
			}
			if bodyHash == "" {
				bodyHash = calculatedBody
			}

			// Получаем секрет
			ki, err := p.Lookup(r, dev, keyID)
			if err != nil || ki == nil || len(ki.Secret) == 0 {
				fail(http.StatusUnauthorized, "unknown key")
				return
			}

			// Canonical string
			canon := strings.Join([]string{
				r.Method,
				r.URL.EscapedPath(),
				r.URL.RawQuery,
				bodyHash,
				"X-OW-Date:" + date,
				"X-OW-Device:" + dev,
				"X-OW-Nonce:" + nonce,
			}, "\n")

			m := hmac.New(sha256.New, ki.Secret)
			m.Write([]byte(canon))
			want := hex.EncodeToString(m.Sum(nil))

			// Сравнение с постоянным временем
			if !hmac.Equal([]byte(strings.ToLower(sigHex)), []byte(strings.ToLower(want))) {
				fail(http.StatusUnauthorized, "invalid signature")
				return
			}

			// Replay-защита
			seen, err := p.SeenNonce(ki.KeyID, nonce, ts)
			if err != nil || seen {
				fail(http.StatusUnauthorized, "replayed nonce")
				return
			}

			// всё ок — пропускаем дальше
			next.ServeHTTP(w, r)
		})
	}
}
