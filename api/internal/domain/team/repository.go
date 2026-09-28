package team

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	// Team CRUD
	// Create inserts the team and the owner's membership in a single transaction.
	Create(ctx context.Context, t *Team) (*Team, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Team, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]*Team, error)
	Update(ctx context.Context, t *Team) (*Team, error)
	SoftDelete(ctx context.Context, teamID, deletedBy uuid.UUID) error

	// Membership — reads
	GetMembership(ctx context.Context, teamID, userID uuid.UUID) (*TeamMember, error)
	GetMemberByID(ctx context.Context, memberID uuid.UUID) (*TeamMember, error)
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error)
	ListJoinRequests(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error)

	// Membership — writes
	InsertMember(ctx context.Context, m *TeamMember) (*TeamMember, error)
	UpdateMember(ctx context.Context, m *TeamMember) (*TeamMember, error)
	DeleteMember(ctx context.Context, memberID uuid.UUID) error

	// Invite links
	CreateInviteLink(ctx context.Context, l *InviteLink) (*InviteLink, error)
	ListInviteLinks(ctx context.Context, teamID uuid.UUID) ([]*InviteLink, error)
	FindInviteLinkByHash(ctx context.Context, tokenHash string) (*InviteLink, error)
	GetInviteLinkByID(ctx context.Context, linkID uuid.UUID) (*InviteLink, error)
	RevokeInviteLink(ctx context.Context, linkID uuid.UUID) error
	IncrementInviteLinkUse(ctx context.Context, linkID uuid.UUID, expiresAt *time.Time) error

	// Email invitations
	CreateInvitation(ctx context.Context, inv *Invitation) (*Invitation, error)
	GetInvitationByID(ctx context.Context, id uuid.UUID) (*Invitation, error)
	FindInvitationByHash(ctx context.Context, tokenHash string) (*Invitation, error)
	FindPendingInvitation(ctx context.Context, teamID uuid.UUID, email string) (*Invitation, error)
	ListPendingInvitations(ctx context.Context, teamID uuid.UUID) ([]*Invitation, error)
	UpdateInvitation(ctx context.Context, inv *Invitation) (*Invitation, error)
	// AcceptInvitation atomically upserts an active team membership for userID and
	// marks the invitation accepted, in a single transaction.
	AcceptInvitation(ctx context.Context, inv *Invitation, userID uuid.UUID) (*TeamMember, error)
}
