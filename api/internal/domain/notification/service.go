package notification

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns notifications for the given user, with optional unread filter.
// Returns (items, hasMore, error). Pagination is handled by the repository via ListParams.
func (s *Service) List(ctx context.Context, p ListParams) ([]*Notification, bool, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}

	// Request one extra item to detect hasMore.
	fetchLimit := p.Limit + 1
	orig := p.Limit
	p.Limit = fetchLimit

	items, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, false, err
	}
	if items == nil {
		return []*Notification{}, false, nil
	}

	hasMore := len(items) > orig
	if hasMore {
		items = items[:orig]
	}
	return items, hasMore, nil
}

func (s *Service) MarkRead(ctx context.Context, id, userID uuid.UUID) (*Notification, error) {
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *Service) Dismiss(ctx context.Context, id, userID uuid.UUID) error {
	return s.repo.Delete(ctx, id, userID)
}

// GetPrefs returns the user's prefs, synthesising defaults if no row exists yet.
func (s *Service) GetPrefs(ctx context.Context, userID uuid.UUID) (*NotificationPrefs, error) {
	prefs, err := s.repo.GetPrefs(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return &NotificationPrefs{
				UserID:        userID,
				EmailEnabled:  true,
				DigestMode:    false,
				DisabledTypes: []string{},
			}, nil
		}
		return nil, err
	}
	return prefs, nil
}

// UpdatePrefs applies a partial update on top of existing (or default) prefs.
func (s *Service) UpdatePrefs(ctx context.Context, in UpdatePrefsInput) (*NotificationPrefs, error) {
	current, err := s.GetPrefs(ctx, in.UserID)
	if err != nil {
		return nil, err
	}

	if in.EmailEnabled != nil {
		current.EmailEnabled = *in.EmailEnabled
	}
	if in.DigestMode != nil {
		current.DigestMode = *in.DigestMode
	}
	if in.DisabledTypes != nil {
		current.DisabledTypes = *in.DisabledTypes
	}
	current.UserID = in.UserID

	return s.repo.UpsertPrefs(ctx, current)
}
