package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a system user who logged in via Google Workspace or a
// magic link. Field/column layout mirrors database/schema/schema.sql's users
// table exactly: one role per user via role_id, no updated_at column.
//
// GoogleSub is nil for a user who has only ever signed in via magic link
// (google_sub is a nullable UNIQUE column - Postgres allows multiple NULLs
// in a UNIQUE column, so this doesn't collide across magic-link-only users).
//
// Department is not set anywhere during login (neither Google nor magic
// link has a notion of it) - it's populated later via
// PATCH /api/admin/users/{userID} (see AdminHandler.UpdateUserProfile) and
// exists primarily so the marksheet PDF can display a student's department
// (see internal/platform/pdf/generator.go).
type User struct {
	ID          uuid.UUID  `json:"id"`
	GoogleSub   *string    `json:"google_sub,omitempty"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	RoleID      uuid.UUID  `json:"role_id"`
	Role        Role       `json:"role"`
	Department  string     `json:"department,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// Role defines the access privileges. Column layout mirrors
// database/schema/schema.sql's roles table (role_name, not name).
type Role struct {
	ID          uuid.UUID `json:"id"`
	Name        RoleName  `json:"name"`
	Description string    `json:"description"`
}
