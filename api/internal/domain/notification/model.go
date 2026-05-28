package notification

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("notification not found")
	ErrForbidden = errors.New("insufficient permission")
)

type Notification struct {
	ID         uuid.UUID      `json:"id"`
	UserID     uuid.UUID      `json:"user_id"`
	Type       string         `json:"type"`
	EntityType string         `json:"entity_type"`
	EntityID   uuid.UUID      `json:"entity_id"`
	Payload    map[string]any `json:"payload"`
	IsRead     bool           `json:"is_read"`
	ReadAt     *time.Time     `json:"read_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// NotificationPrefs represents per-user notification preferences.
type NotificationPrefs struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	EmailEnabled  bool      `json:"email_enabled"`
	DigestMode    bool      `json:"digest_mode"`
	DisabledTypes []string  `json:"disabled_types"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ListParams controls pagination for listing notifications.
type ListParams struct {
	UserID    uuid.UUID
	UnreadOnly bool
	Limit     int
	Cursor    string // opaque: ISO timestamp of last seen created_at
}

// UpdatePrefsInput carries fields the caller may change.
type UpdatePrefsInput struct {
	UserID        uuid.UUID
	EmailEnabled  *bool
	DigestMode    *bool
	DisabledTypes *[]string
}
