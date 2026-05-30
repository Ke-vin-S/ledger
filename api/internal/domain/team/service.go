package team

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
	"github.com/Ke-vin-S/ledger/api/internal/domain/user"
)

type Service struct {
	repo     Repository
	userRepo user.Repository
	auditor  audit.Logger
}

func NewService(repo Repository, userRepo user.Repository, auditor audit.Logger) *Service {
	return &Service{repo: repo, userRepo: userRepo, auditor: auditor}
}

// Create creates a team and adds the creator as owner in a single transaction.
func (s *Service) Create(ctx context.Context, createdBy uuid.UUID, name, description, currency string, isPublic bool) (*Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return nil, fmt.Errorf("currency must be a 3-character ISO 4217 code")
	}
	var desc *string
	if d := strings.TrimSpace(description); d != "" {
		desc = &d
	}
	t := &Team{
		Name:        name,
		Description: desc,
		Currency:    currency,
		IsPublic:    isPublic,
		OwnerID:     createdBy,
		CreatedBy:   createdBy,
	}
	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionTeamCreated,
		ActorID:    &createdBy,
		TeamID:     &created.ID,
		EntityType: "team",
		EntityID:   created.ID,
		After:      created,
	})
	return created, nil
}

// GetForMember returns the team if the requester is an active member.
func (s *Service) GetForMember(ctx context.Context, teamID, requesterID uuid.UUID) (*Team, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleMember); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, teamID)
}

// ListForUser returns all teams the user is an active member of.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]*Team, error) {
	return s.repo.ListForUser(ctx, userID)
}

// Update updates mutable team fields. Requires admin+.
func (s *Service) Update(ctx context.Context, teamID, requesterID uuid.UUID, name, description *string, isPublic *bool) (*Team, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return nil, err
	}
	t, err := s.repo.FindByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		t.Name = n
	}
	if description != nil {
		d := strings.TrimSpace(*description)
		t.Description = &d
	}
	if isPublic != nil {
		t.IsPublic = *isPublic
	}
	updated, err := s.repo.Update(ctx, t)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionTeamUpdated,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team",
		EntityID:   teamID,
		After:      updated,
	})
	return updated, nil
}

// Delete soft-deletes a team. Requires owner.
func (s *Service) Delete(ctx context.Context, teamID, requesterID uuid.UUID) error {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleOwner); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, teamID, requesterID); err != nil {
		return err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionTeamDeleted,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team",
		EntityID:   teamID,
	})
	return nil
}

// ListMembers returns all members of a team. Requires member+.
func (s *Service) ListMembers(ctx context.Context, teamID, requesterID uuid.UUID) ([]*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleMember); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, teamID)
}

// InviteMember invites a registered user by email. Requires admin+.
func (s *Service) InviteMember(ctx context.Context, teamID, inviterID uuid.UUID, email string) (*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, inviterID, RoleAdmin); err != nil {
		return nil, err
	}
	invitee, err := s.userRepo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, user.ErrNotFound
	}
	return s.upsertInvitation(ctx, teamID, invitee.ID, inviterID)
}

// AddAnonymousMember adds an existing anonymous user to the team as an active member. Requires member+.
// Anonymous users cannot interactively accept invitations, so they are added as active immediately.
func (s *Service) AddAnonymousMember(ctx context.Context, teamID, requesterID, anonUserID uuid.UUID) (*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleMember); err != nil {
		return nil, err
	}
	now := time.Now()
	existing, err := s.repo.GetMembership(ctx, teamID, anonUserID)
	if err == nil {
		switch existing.Status {
		case StatusActive, StatusInvited, StatusRequested:
			return nil, ErrAlreadyMember
		}
		existing.Status = StatusActive
		existing.InvitedBy = &requesterID
		existing.ResolvedBy = nil
		existing.ResolvedAt = nil
		existing.JoinedAt = &now
		m, err := s.repo.UpdateMember(ctx, existing)
		if err != nil {
			return nil, err
		}
		_ = s.auditor.Log(ctx, audit.Entry{
			Action:     audit.ActionMemberInvited,
			ActorID:    &requesterID,
			TeamID:     &teamID,
			EntityType: "team_member",
			EntityID:   m.ID,
		})
		return m, nil
	}
	m := &TeamMember{
		TeamID:    teamID,
		UserID:    anonUserID,
		Role:      RoleMember,
		Status:    StatusActive,
		InvitedBy: &requesterID,
		JoinedAt:  &now,
	}
	created, err := s.repo.InsertMember(ctx, m)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberInvited,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   created.ID,
	})
	return created, nil
}

// upsertInvitation handles both new invitations and re-invitations after removal.
func (s *Service) upsertInvitation(ctx context.Context, teamID, userID, inviterID uuid.UUID) (*TeamMember, error) {
	existing, err := s.repo.GetMembership(ctx, teamID, userID)
	if err == nil {
		switch existing.Status {
		case StatusActive, StatusInvited, StatusRequested:
			return nil, ErrAlreadyMember
		}
		// Re-invite after removed/rejected/left.
		existing.Status = StatusInvited
		existing.InvitedBy = &inviterID
		existing.ResolvedBy = nil
		existing.ResolvedAt = nil
		m, err := s.repo.UpdateMember(ctx, existing)
		if err != nil {
			return nil, err
		}
		_ = s.auditor.Log(ctx, audit.Entry{
			Action:     audit.ActionMemberInvited,
			ActorID:    &inviterID,
			TeamID:     &teamID,
			EntityType: "team_member",
			EntityID:   m.ID,
		})
		return m, nil
	}
	m := &TeamMember{
		TeamID:    teamID,
		UserID:    userID,
		Role:      RoleMember,
		Status:    StatusInvited,
		InvitedBy: &inviterID,
	}
	created, err := s.repo.InsertMember(ctx, m)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberInvited,
		ActorID:    &inviterID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   created.ID,
	})
	return created, nil
}

// RequestJoin lets a user request to join a public team.
func (s *Service) RequestJoin(ctx context.Context, teamID, requesterID uuid.UUID, message string) (*TeamMember, error) {
	t, err := s.repo.FindByID(ctx, teamID)
	if err != nil {
		return nil, ErrNotFound
	}
	if !t.IsPublic {
		return nil, ErrTeamNotPublic
	}
	existing, err := s.repo.GetMembership(ctx, teamID, requesterID)
	if err == nil {
		if existing.Status == StatusActive || existing.Status == StatusInvited || existing.Status == StatusRequested {
			return nil, ErrAlreadyMember
		}
	}
	var msg *string
	if m := strings.TrimSpace(message); m != "" {
		msg = &m
	}
	m := &TeamMember{
		TeamID:         teamID,
		UserID:         requesterID,
		Role:           RoleMember,
		Status:         StatusRequested,
		RequestMessage: msg,
	}
	created, err := s.repo.InsertMember(ctx, m)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberRequested,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   created.ID,
	})
	return created, nil
}

// ListJoinRequests lists pending join requests. Requires admin+.
func (s *Service) ListJoinRequests(ctx context.Context, teamID, requesterID uuid.UUID) ([]*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListJoinRequests(ctx, teamID)
}

// ApproveJoin approves a join request. Requires admin+.
func (s *Service) ApproveJoin(ctx context.Context, teamID uuid.UUID, requestID, approverID uuid.UUID) (*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, approverID, RoleAdmin); err != nil {
		return nil, err
	}
	req, err := s.repo.GetMemberByID(ctx, requestID)
	if err != nil || req.Status != StatusRequested {
		return nil, fmt.Errorf("join request not found or already resolved")
	}
	now := time.Now()
	req.Status = StatusActive
	req.ResolvedBy = &approverID
	req.ResolvedAt = &now
	req.JoinedAt = &now
	m, err := s.repo.UpdateMember(ctx, req)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberApproved,
		ActorID:    &approverID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   m.ID,
	})
	return m, nil
}

// RejectJoin rejects a join request. Requires admin+.
func (s *Service) RejectJoin(ctx context.Context, teamID uuid.UUID, requestID, approverID uuid.UUID) (*TeamMember, error) {
	if _, err := s.requireMembership(ctx, teamID, approverID, RoleAdmin); err != nil {
		return nil, err
	}
	req, err := s.repo.GetMemberByID(ctx, requestID)
	if err != nil || req.Status != StatusRequested {
		return nil, fmt.Errorf("join request not found or already resolved")
	}
	now := time.Now()
	req.Status = StatusRejected
	req.ResolvedBy = &approverID
	req.ResolvedAt = &now
	m, err := s.repo.UpdateMember(ctx, req)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberRejected,
		ActorID:    &approverID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   m.ID,
	})
	return m, nil
}

// ChangeRole changes a member's role. Complex permission matrix:
//   - owner can set any role on any member
//   - admin can promote member→admin only
func (s *Service) ChangeRole(ctx context.Context, teamID, targetUserID, requesterID uuid.UUID, newRole string) (*TeamMember, error) {
	requester, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin)
	if err != nil {
		return nil, err
	}
	target, err := s.repo.GetMembership(ctx, teamID, targetUserID)
	if err != nil {
		return nil, ErrNotMember
	}
	if target.Status != StatusActive {
		return nil, fmt.Errorf("member is not active")
	}
	if err := validateRoleChange(requester.Role, target.Role, newRole); err != nil {
		return nil, err
	}
	target.Role = newRole
	// Transfer ownership: update team.owner_id too.
	if newRole == RoleOwner {
		t, err := s.repo.FindByID(ctx, teamID)
		if err != nil {
			return nil, err
		}
		t.OwnerID = targetUserID
		if _, err := s.repo.Update(ctx, t); err != nil {
			return nil, fmt.Errorf("update team owner: %w", err)
		}
		// Demote the previous owner to admin.
		prev, err := s.repo.GetMembership(ctx, teamID, requesterID)
		if err == nil && prev.Role == RoleOwner {
			prev.Role = RoleAdmin
			if _, err := s.repo.UpdateMember(ctx, prev); err != nil {
				return nil, fmt.Errorf("demote previous owner: %w", err)
			}
		}
	}
	m, err := s.repo.UpdateMember(ctx, target)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// RemoveMember removes a member or lets a user leave. Rules:
//   - admin+ can remove non-owner active members
//   - any member can remove themselves (leave)
func (s *Service) RemoveMember(ctx context.Context, teamID, targetUserID, requesterID uuid.UUID) error {
	requester, err := s.requireMembership(ctx, teamID, requesterID, RoleMember)
	if err != nil {
		return err
	}
	isSelf := targetUserID == requesterID
	target, err := s.repo.GetMembership(ctx, teamID, targetUserID)
	if err != nil {
		return ErrNotMember
	}
	if target.Role == RoleOwner {
		return ErrCannotRemoveOwner
	}
	if !isSelf && !RoleAtLeast(requester.Role, RoleAdmin) {
		return ErrInsufficientRole
	}
	now := time.Now()
	if isSelf {
		target.Status = StatusLeft
	} else {
		target.Status = StatusRemoved
	}
	target.ResolvedBy = &requesterID
	target.ResolvedAt = &now
	if _, err := s.repo.UpdateMember(ctx, target); err != nil {
		return err
	}
	action := audit.ActionMemberRemoved
	if isSelf {
		action = audit.ActionMemberLeft
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     action,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team_member",
		EntityID:   target.ID,
	})
	return nil
}

// CreateInviteLink generates an invite link. Requires admin+.
// Returns the InviteLink and the raw token (to embed in the URL).
func (s *Service) CreateInviteLink(ctx context.Context, teamID, createdBy uuid.UUID, maxUses *int, expiresInHours *int) (*InviteLink, string, error) {
	if _, err := s.requireMembership(ctx, teamID, createdBy, RoleAdmin); err != nil {
		return nil, "", err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	rawToken := hex.EncodeToString(raw)
	h := hashLinkToken(rawToken)

	var expiresAt *time.Time
	if expiresInHours != nil && *expiresInHours > 0 {
		t := time.Now().Add(time.Duration(*expiresInHours) * time.Hour)
		expiresAt = &t
	}
	link := &InviteLink{
		TeamID:    teamID,
		CreatedBy: createdBy,
		TokenHash: h,
		MaxUses:   maxUses,
		ExpiresAt: expiresAt,
	}
	created, err := s.repo.CreateInviteLink(ctx, link)
	if err != nil {
		return nil, "", err
	}
	return created, rawToken, nil
}

// ListInviteLinks returns active (non-revoked) invite links. Requires admin+.
func (s *Service) ListInviteLinks(ctx context.Context, teamID, requesterID uuid.UUID) ([]*InviteLink, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListInviteLinks(ctx, teamID)
}

// RevokeInviteLink revokes an invite link. Requires admin+.
func (s *Service) RevokeInviteLink(ctx context.Context, teamID, linkID, requesterID uuid.UUID) error {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return err
	}
	return s.repo.RevokeInviteLink(ctx, linkID)
}

// JoinViaInviteLink adds the user to the team via an invite link token.
func (s *Service) JoinViaInviteLink(ctx context.Context, rawToken string, userID uuid.UUID) (*TeamMember, error) {
	h := hashLinkToken(rawToken)
	link, err := s.repo.FindInviteLinkByHash(ctx, h)
	if err != nil {
		return nil, ErrInviteLinkInvalid
	}
	if link.RevokedAt != nil {
		return nil, ErrInviteLinkInvalid
	}
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, ErrInviteLinkInvalid
	}
	if link.MaxUses != nil && link.UseCount >= *link.MaxUses {
		return nil, ErrInviteLinkInvalid
	}

	// Check if already an active member.
	existing, err := s.repo.GetMembership(ctx, link.TeamID, userID)
	if err == nil && (existing.Status == StatusActive || existing.Status == StatusInvited) {
		return nil, ErrAlreadyMember
	}

	now := time.Now()
	var m *TeamMember
	if err == nil {
		// Re-activate if previously removed/left/rejected.
		existing.Status = StatusActive
		existing.JoinedAt = &now
		m, err = s.repo.UpdateMember(ctx, existing)
	} else {
		member := &TeamMember{
			TeamID:   link.TeamID,
			UserID:   userID,
			Role:     RoleMember,
			Status:   StatusActive,
			JoinedAt: &now,
		}
		m, err = s.repo.InsertMember(ctx, member)
	}
	if err != nil {
		return nil, err
	}
	if err := s.repo.IncrementInviteLinkUse(ctx, link.ID, link.ExpiresAt); err != nil {
		return nil, fmt.Errorf("increment use count: %w", err)
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberInvited,
		ActorID:    &userID,
		TeamID:     &link.TeamID,
		EntityType: "team_member",
		EntityID:   m.ID,
		Meta:       map[string]any{"via_invite_link": link.ID.String()},
	})
	return m, nil
}

// requireMembership fetches the caller's active membership and checks minimum role.
func (s *Service) requireMembership(ctx context.Context, teamID, userID uuid.UUID, minRole string) (*TeamMember, error) {
	m, err := s.repo.GetMembership(ctx, teamID, userID)
	if err != nil {
		return nil, ErrNotMember
	}
	if m.Status != StatusActive {
		return nil, ErrNotMember
	}
	if !RoleAtLeast(m.Role, minRole) {
		return nil, ErrInsufficientRole
	}
	return m, nil
}

func validateRoleChange(requesterRole, targetCurrentRole, newRole string) error {
	if requesterRole == RoleOwner {
		return nil // owner can do anything
	}
	// Admin can only promote member→admin.
	if requesterRole == RoleAdmin && targetCurrentRole == RoleMember && newRole == RoleAdmin {
		return nil
	}
	return ErrInsufficientRole
}

func hashLinkToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
