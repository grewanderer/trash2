package models

import (
	"time"

	"github.com/google/uuid"
)

// internal/auth/models.go
type Organization struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

type User struct {
	ID        uuid.UUID `db:"id"`
	OrgID     uuid.UUID `db:"org_id"`
	Email     string    `db:"email"`
	Password  []byte    `db:"password_hash"`
	Role      string    `db:"role"` // owner|manager|tech|viewer
	CreatedAt time.Time `db:"created_at"`
}

type APIToken struct {
	ID        uuid.UUID  `db:"id"`
	OrgID     uuid.UUID  `db:"org_id"`
	UserID    uuid.UUID  `db:"user_id"`
	TokenHash []byte     `db:"token_hash"`
	Scope     string     `db:"scope"`
	ExpiresAt *time.Time `db:"expires_at"`
}
