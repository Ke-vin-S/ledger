package auditlog

import (
	"time"

	"github.com/google/uuid"
)

// LogEntry is the read model for a single audit_log row.
type LogEntry struct {
	ID         uuid.UUID      `json:"id"`
	Action     string         `json:"action"`
	ActorID    *uuid.UUID     `json:"actor_id,omitempty"`
	TeamID     *uuid.UUID     `json:"team_id,omitempty"`
	EntityType string         `json:"entity_type"`
	EntityID   uuid.UUID      `json:"entity_id"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ListParams controls pagination and optional action filter.
type ListParams struct {
	Limit  int
	Cursor string // opaque: ISO timestamp of last-seen created_at
	Action string // optional filter, e.g. "expense.created"
}
