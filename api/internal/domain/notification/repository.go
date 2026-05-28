package notification

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	// Notification CRUD
	List(ctx context.Context, p ListParams) ([]*Notification, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Notification, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) (*Notification, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	Delete(ctx context.Context, id, userID uuid.UUID) error

	// Preferences
	GetPrefs(ctx context.Context, userID uuid.UUID) (*NotificationPrefs, error)
	UpsertPrefs(ctx context.Context, prefs *NotificationPrefs) (*NotificationPrefs, error)
}
