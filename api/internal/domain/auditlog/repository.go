package auditlog

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListByTeam(ctx context.Context, teamID uuid.UUID, p ListParams) ([]*LogEntry, error)
	ListByActor(ctx context.Context, actorID uuid.UUID, p ListParams) ([]*LogEntry, error)
}
