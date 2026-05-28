package auditlog

import (
	"context"

	"github.com/google/uuid"
)

const defaultLimit = 20

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListTeamEntries returns paginated audit entries for a team, newest first.
// Returns (items, hasMore, error).
func (s *Service) ListTeamEntries(ctx context.Context, teamID uuid.UUID, p ListParams) ([]*LogEntry, bool, error) {
	p = withDefaultLimit(p)

	// Fetch limit+1 to detect a next page.
	fetch := p
	fetch.Limit = p.Limit + 1

	entries, err := s.repo.ListByTeam(ctx, teamID, fetch)
	if err != nil {
		return nil, false, err
	}

	return paginate(entries, p.Limit)
}

// ListMyEntries returns paginated audit entries where the caller is the actor.
func (s *Service) ListMyEntries(ctx context.Context, actorID uuid.UUID, p ListParams) ([]*LogEntry, bool, error) {
	p = withDefaultLimit(p)

	fetch := p
	fetch.Limit = p.Limit + 1

	entries, err := s.repo.ListByActor(ctx, actorID, fetch)
	if err != nil {
		return nil, false, err
	}

	return paginate(entries, p.Limit)
}

func withDefaultLimit(p ListParams) ListParams {
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	return p
}

func paginate(entries []*LogEntry, limit int) ([]*LogEntry, bool, error) {
	if entries == nil {
		return []*LogEntry{}, false, nil
	}
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	return entries, hasMore, nil
}
