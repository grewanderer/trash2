package owctrl

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// Очень простой вариант: Authorization: Bearer <sharedSecret>
func sharedSecretAuth(secret string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				next.ServeHTTP(w, r)
				return
			}
			const p = "Bearer "
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, p) || strings.TrimPrefix(auth, p) != secret {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func SharedSecretAuth(secret string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				next.ServeHTTP(w, r)
				return
			}
			const p = "Bearer "
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, p) || strings.TrimPrefix(auth, p) != secret {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
