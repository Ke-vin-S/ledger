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

// Mailer sends the transactional emails this service triggers.
// Implemented by *email.Mailer; failures are best-effort and never fail the request.
type Mailer interface {
	InvitationEmail(ctx context.Context, to, teamName, inviterName, rawToken string) error
	JoinApproved(ctx context.Context, to, userName, teamName string) error
	JoinRejected(ctx context.Context, to, userName, teamName string) error
	InviteLink(ctx context.Context, to, teamName, rawToken string) error
}

type Service struct {
	repo     Repository
	userRepo user.Repository
	auditor  audit.Logger
	mailer   Mailer
}

func NewService(repo Repository, userRepo user.Repository, auditor audit.Logger, mailer Mailer) *Service {
	return &Service{repo: repo, userRepo: userRepo, auditor: auditor, mailer: mailer}
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

// invitationTTL is how long a pending invitation remains valid.
const invitationTTL = 7 * 24 * time.Hour

// InviteByEmail creates a pending email invitation and emails an accept link.
// Works whether or not the email already belongs to a user. Requires admin+.
// The invitee joins as a plain member when they accept.
func (s *Service) InviteByEmail(ctx context.Context, teamID, inviterID uuid.UUID, email string) (*Invitation, error) {
	if _, err := s.requireMembership(ctx, teamID, inviterID, RoleAdmin); err != nil {
		return nil, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !looksLikeEmail(email) {
		return nil, ErrInvalidEmail
	}

	// Reject if the email already belongs to an active member.
	if u, err := s.userRepo.FindByEmail(ctx, email); err == nil {
		if m, err := s.repo.GetMembership(ctx, teamID, u.ID); err == nil && m.Status == StatusActive {
			return nil, ErrAlreadyMember
		}
	}
	// Reject if there is already a pending invitation for this email.
	if _, err := s.repo.FindPendingInvitation(ctx, teamID, email); err == nil {
		return nil, ErrInvitationExists
	}

	rawToken, tokenHash, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	inv := &Invitation{
		TeamID:    teamID,
		Email:     email,
		Role:      RoleMember,
		Status:    InvitationPending,
		TokenHash: tokenHash,
		InvitedBy: inviterID,
		ExpiresAt: time.Now().Add(invitationTTL),
	}
	created, err := s.repo.CreateInvitation(ctx, inv)
	if err != nil {
		return nil, err
	}
	s.emailInvitation(ctx, created, rawToken)
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberInvited,
		ActorID:    &inviterID,
		TeamID:     &teamID,
		EntityType: "team_invitation",
		EntityID:   created.ID,
		Meta:       map[string]any{"email": email},
	})
	return created, nil
}

// ListInvitations returns pending invitations for a team. Requires admin+.
func (s *Service) ListInvitations(ctx context.Context, teamID, requesterID uuid.UUID) ([]*Invitation, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListPendingInvitations(ctx, teamID)
}

// CancelInvitation revokes a pending invitation. Requires admin+.
func (s *Service) CancelInvitation(ctx context.Context, teamID, invitationID, requesterID uuid.UUID) error {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return err
	}
	inv, err := s.repo.GetInvitationByID(ctx, invitationID)
	if err != nil || inv.TeamID != teamID {
		return ErrInvitationNotFound
	}
	if inv.Status != InvitationPending {
		return ErrInvitationInvalid
	}
	now := time.Now()
	inv.Status = InvitationCancelled
	inv.CancelledBy = &requesterID
	inv.CancelledAt = &now
	if _, err := s.repo.UpdateInvitation(ctx, inv); err != nil {
		return err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberRejected,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team_invitation",
		EntityID:   inv.ID,
	})
	return nil
}

// ResendInvitation rotates the token, resets the expiry, and re-emails a pending
// invitation. Requires admin+.
func (s *Service) ResendInvitation(ctx context.Context, teamID, invitationID, requesterID uuid.UUID) (*Invitation, error) {
	if _, err := s.requireMembership(ctx, teamID, requesterID, RoleAdmin); err != nil {
		return nil, err
	}
	inv, err := s.repo.GetInvitationByID(ctx, invitationID)
	if err != nil || inv.TeamID != teamID {
		return nil, ErrInvitationNotFound
	}
	if inv.Status != InvitationPending {
		return nil, ErrInvitationInvalid
	}
	rawToken, tokenHash, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	inv.TokenHash = tokenHash
	inv.ExpiresAt = time.Now().Add(invitationTTL)
	updated, err := s.repo.UpdateInvitation(ctx, inv)
	if err != nil {
		return nil, err
	}
	s.emailInvitation(ctx, updated, rawToken)
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberInvited,
		ActorID:    &requesterID,
		TeamID:     &teamID,
		EntityType: "team_invitation",
		EntityID:   updated.ID,
		Meta:       map[string]any{"resent": true},
	})
	return updated, nil
}

// AcceptInvitation consumes an invitation token and makes the authenticated user
// an active member. The invitation's token is the secret — the accepting user's
// email need not match the invited address.
func (s *Service) AcceptInvitation(ctx context.Context, rawToken string, userID uuid.UUID) (*TeamMember, error) {
	inv, err := s.repo.FindInvitationByHash(ctx, hashLinkToken(rawToken))
	if err != nil {
		return nil, ErrInvitationInvalid
	}
	if inv.Status != InvitationPending || time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationInvalid
	}
	if m, err := s.repo.GetMembership(ctx, inv.TeamID, userID); err == nil && m.Status == StatusActive {
		return nil, ErrAlreadyMember
	}
	m, err := s.repo.AcceptInvitation(ctx, inv, userID)
	if err != nil {
		return nil, err
	}
	_ = s.auditor.Log(ctx, audit.Entry{
		Action:     audit.ActionMemberApproved,
		ActorID:    &userID,
		TeamID:     &inv.TeamID,
		EntityType: "team_member",
		EntityID:   m.ID,
		Meta:       map[string]any{"via_invitation": inv.ID.String()},
	})
	return m, nil
}

// emailInvitation best-effort emails the accept link. Never fails the request.
func (s *Service) emailInvitation(ctx context.Context, inv *Invitation, rawToken string) {
	if s.mailer == nil {
		return
	}
	t, err := s.repo.FindByID(ctx, inv.TeamID)
	if err != nil {
		return
	}
	inviterName := "A team admin"
	if u, err := s.userRepo.FindByID(ctx, inv.InvitedBy); err == nil {
		inviterName = u.DisplayName
	}
	_ = s.mailer.InvitationEmail(ctx, inv.Email, t.Name, inviterName, rawToken)
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
	s.sendJoinDecisionEmail(ctx, teamID, m.UserID, true)
	return m, nil
}

// sendJoinDecisionEmail best-effort notifies a requester of an approve/reject decision.
func (s *Service) sendJoinDecisionEmail(ctx context.Context, teamID, userID uuid.UUID, approved bool) {
	if s.mailer == nil {
		return
	}
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || u.Email == nil {
		return
	}
	t, err := s.repo.FindByID(ctx, teamID)
	if err != nil {
		return
	}
	if approved {
		_ = s.mailer.JoinApproved(ctx, *u.Email, u.DisplayName, t.Name)
	} else {
		_ = s.mailer.JoinRejected(ctx, *u.Email, u.DisplayName, t.Name)
	}
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
	s.sendJoinDecisionEmail(ctx, teamID, m.UserID, false)
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
// If email is non-nil and non-empty, the freshly-created link is emailed to it
// (best-effort — email failure does not fail link creation).
func (s *Service) CreateInviteLink(ctx context.Context, teamID, createdBy uuid.UUID, maxUses *int, expiresInHours *int, email *string) (*InviteLink, string, error) {
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
	if s.mailer != nil && email != nil {
		if to := strings.TrimSpace(*email); to != "" {
			if t, err := s.repo.FindByID(ctx, teamID); err == nil {
				_ = s.mailer.InviteLink(ctx, to, t.Name, rawToken)
			}
		}
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

// newInviteToken returns a random raw token and its sha256 hash for storage.
func newInviteToken() (raw, hash string, err error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	raw = hex.EncodeToString(b)
	return raw, hashLinkToken(raw), nil
}

// looksLikeEmail is a minimal sanity check — real validation happens on delivery.
func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && !strings.ContainsAny(s, " \t")
}
