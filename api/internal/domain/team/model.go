package team

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Role constants
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Status constants
const (
	StatusActive    = "active"
	StatusInvited   = "invited"
	StatusRequested = "requested"
	StatusRejected  = "rejected"
	StatusRemoved   = "removed"
	StatusLeft      = "left"
)

var roleOrder = map[string]int{RoleMember: 1, RoleAdmin: 2, RoleOwner: 3}

// RoleAtLeast reports whether actual is at least as privileged as required.
func RoleAtLeast(actual, required string) bool {
	return roleOrder[actual] >= roleOrder[required]
}

type Team struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Currency    string
	IsPublic    bool
	OwnerID     uuid.UUID
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	DeletedAt   *time.Time
	DeletedBy   *uuid.UUID
}

type TeamMember struct {
	ID             uuid.UUID
	TeamID         uuid.UUID
	UserID         uuid.UUID
	Role           string
	Status         string
	InvitedBy      *uuid.UUID
	RequestMessage *string
	ResolvedBy     *uuid.UUID
	ResolvedAt     *time.Time
	JoinedAt       *time.Time
	CreatedAt      time.Time

	// Populated on list/get reads via JOIN — empty on writes.
	UserDisplayName  string
	UserAvatarURL    *string
	UserIdentityType string
}

type InviteLink struct {
	ID        uuid.UUID
	TeamID    uuid.UUID
	CreatedBy uuid.UUID
	TokenHash string
	MaxUses   *int
	UseCount  int
	ExpiresAt *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Domain errors
var (
	ErrNotFound        = errors.New("team not found")
	ErrNotMember       = errors.New("not a team member")
	ErrInsufficientRole = errors.New("insufficient role for this action")
	ErrAlreadyMember   = errors.New("user is already a member of this team")
	ErrTeamNotPublic   = errors.New("team is not open to join requests")
	ErrInviteLinkInvalid   = errors.New("invite link is invalid, expired, or exhausted")
	ErrCannotRemoveOwner   = errors.New("cannot remove the team owner")
	ErrOwnerRoleTransfer   = errors.New("to transfer ownership use the role-change endpoint with role=owner")
)
