package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ke-vin-S/ledger/api/internal/domain/team"
)

type teamRepo struct {
	pool *pgxpool.Pool
}

func NewTeamRepo(pool *pgxpool.Pool) team.Repository {
	return &teamRepo{pool: pool}
}

// ── Team CRUD ────────────────────────────────────────────────────────────────

func (r *teamRepo) Create(ctx context.Context, t *team.Team) (*team.Team, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		INSERT INTO teams (name, description, currency, is_public, owner_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, currency, is_public, owner_id, created_by, created_at, deleted_at, deleted_by
	`, t.Name, t.Description, t.Currency, t.IsPublic, t.OwnerID, t.CreatedBy)

	created, err := scanTeam(row)
	if err != nil {
		return nil, fmt.Errorf("insert team: %w", err)
	}

	now := time.Now()
	_, err = tx.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role, status, joined_at)
		VALUES ($1, $2, 'owner', 'active', $3)
	`, created.ID, t.OwnerID, now)
	if err != nil {
		return nil, fmt.Errorf("insert owner membership: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return created, nil
}

func (r *teamRepo) FindByID(ctx context.Context, id uuid.UUID) (*team.Team, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, description, currency, is_public, owner_id, created_by, created_at, deleted_at, deleted_by
		FROM teams WHERE id = $1 AND deleted_at IS NULL
	`, id)
	t, err := scanTeam(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *teamRepo) ListForUser(ctx context.Context, userID uuid.UUID) ([]*team.Team, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.description, t.currency, t.is_public, t.owner_id, t.created_by, t.created_at, t.deleted_at, t.deleted_by
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1 AND tm.status = 'active' AND t.deleted_at IS NULL
		ORDER BY t.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTeams(rows)
}

func (r *teamRepo) Update(ctx context.Context, t *team.Team) (*team.Team, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE teams
		SET name = $1, description = $2, is_public = $3, owner_id = $4
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING id, name, description, currency, is_public, owner_id, created_by, created_at, deleted_at, deleted_by
	`, t.Name, t.Description, t.IsPublic, t.OwnerID, t.ID)
	updated, err := scanTeam(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrNotFound
		}
		return nil, err
	}
	return updated, nil
}

func (r *teamRepo) SoftDelete(ctx context.Context, teamID, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE teams SET deleted_at = NOW(), deleted_by = $1
		WHERE id = $2 AND deleted_at IS NULL
	`, deletedBy, teamID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return team.ErrNotFound
	}
	return nil
}

// ── Membership reads ─────────────────────────────────────────────────────────

func (r *teamRepo) GetMembership(ctx context.Context, teamID, userID uuid.UUID) (*team.TeamMember, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.status,
		       tm.invited_by, tm.request_message, tm.resolved_by, tm.resolved_at,
		       tm.joined_at, tm.created_at,
		       u.display_name, u.avatar_url, u.identity_type
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1 AND tm.user_id = $2
	`, teamID, userID)
	return scanMember(row)
}

func (r *teamRepo) GetMemberByID(ctx context.Context, memberID uuid.UUID) (*team.TeamMember, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.status,
		       tm.invited_by, tm.request_message, tm.resolved_by, tm.resolved_at,
		       tm.joined_at, tm.created_at,
		       u.display_name, u.avatar_url, u.identity_type
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.id = $1
	`, memberID)
	return scanMember(row)
}

func (r *teamRepo) ListMembers(ctx context.Context, teamID uuid.UUID) ([]*team.TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.status,
		       tm.invited_by, tm.request_message, tm.resolved_by, tm.resolved_at,
		       tm.joined_at, tm.created_at,
		       u.display_name, u.avatar_url, u.identity_type
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1 AND tm.status IN ('active', 'invited')
		ORDER BY tm.joined_at ASC NULLS LAST, tm.created_at ASC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMembers(rows)
}

func (r *teamRepo) ListJoinRequests(ctx context.Context, teamID uuid.UUID) ([]*team.TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tm.id, tm.team_id, tm.user_id, tm.role, tm.status,
		       tm.invited_by, tm.request_message, tm.resolved_by, tm.resolved_at,
		       tm.joined_at, tm.created_at,
		       u.display_name, u.avatar_url, u.identity_type
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1 AND tm.status = 'requested'
		ORDER BY tm.created_at ASC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMembers(rows)
}

// ── Membership writes ────────────────────────────────────────────────────────

func (r *teamRepo) InsertMember(ctx context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin member tx: %w", err)
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		INSERT INTO team_members (team_id, user_id, role, status, invited_by, request_message, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, team_id, user_id, role, status,
		          invited_by, request_message, resolved_by, resolved_at, joined_at, created_at
	`, m.TeamID, m.UserID, m.Role, m.Status, m.InvitedBy, m.RequestMessage, m.JoinedAt)
	created, err := scanMemberCore(row)
	if err != nil {
		return nil, err
	}
	if m.Status == team.StatusRequested {
		admins, err := teamAdminIDs(ctx, tx, m.TeamID)
		if err != nil {
			return nil, err
		}
		if err := createTx(ctx, tx, admins, "team.join_requested", "team_member", created.ID, map[string]any{
			"team_id": m.TeamID,
			"user_id": m.UserID,
			"message": m.RequestMessage,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit member: %w", err)
	}
	return created, nil
}

func (r *teamRepo) UpdateMember(ctx context.Context, m *team.TeamMember) (*team.TeamMember, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE team_members
		SET role = $1, status = $2, invited_by = $3, request_message = $4,
		    resolved_by = $5, resolved_at = $6, joined_at = $7
		WHERE id = $8
		RETURNING id, team_id, user_id, role, status,
		          invited_by, request_message, resolved_by, resolved_at, joined_at, created_at
	`, m.Role, m.Status, m.InvitedBy, m.RequestMessage,
		m.ResolvedBy, m.ResolvedAt, m.JoinedAt, m.ID)
	return scanMemberCore(row)
}

func (r *teamRepo) DeleteMember(ctx context.Context, memberID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM team_members WHERE id = $1`, memberID)
	return err
}

// ── Invite links ─────────────────────────────────────────────────────────────

func (r *teamRepo) CreateInviteLink(ctx context.Context, l *team.InviteLink) (*team.InviteLink, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO invite_links (team_id, created_by, token_hash, max_uses, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, team_id, created_by, token_hash, max_uses, use_count, expires_at, revoked_at, created_at
	`, l.TeamID, l.CreatedBy, l.TokenHash, l.MaxUses, l.ExpiresAt)
	return scanInviteLink(row)
}

func (r *teamRepo) ListInviteLinks(ctx context.Context, teamID uuid.UUID) ([]*team.InviteLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, created_by, token_hash, max_uses, use_count, expires_at, revoked_at, created_at
		FROM invite_links
		WHERE team_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []*team.InviteLink
	for rows.Next() {
		l, err := scanInviteLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (r *teamRepo) FindInviteLinkByHash(ctx context.Context, tokenHash string) (*team.InviteLink, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, team_id, created_by, token_hash, max_uses, use_count, expires_at, revoked_at, created_at
		FROM invite_links WHERE token_hash = $1
	`, tokenHash)
	l, err := scanInviteLink(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrInviteLinkInvalid
		}
		return nil, err
	}
	return l, nil
}

func (r *teamRepo) GetInviteLinkByID(ctx context.Context, linkID uuid.UUID) (*team.InviteLink, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, team_id, created_by, token_hash, max_uses, use_count, expires_at, revoked_at, created_at
		FROM invite_links WHERE id = $1
	`, linkID)
	l, err := scanInviteLink(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrInviteLinkInvalid
		}
		return nil, err
	}
	return l, nil
}

func (r *teamRepo) RevokeInviteLink(ctx context.Context, linkID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE invite_links SET revoked_at = NOW() WHERE id = $1`, linkID)
	return err
}

func (r *teamRepo) IncrementInviteLinkUse(ctx context.Context, linkID uuid.UUID, _ *time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE invite_links SET use_count = use_count + 1 WHERE id = $1`, linkID)
	return err
}

// ── Email invitations ────────────────────────────────────────────────────────

const invitationCols = `id, team_id, email, role, status, token_hash, invited_by,
	expires_at, accepted_by, accepted_at, cancelled_by, cancelled_at, created_at`

func (r *teamRepo) CreateInvitation(ctx context.Context, inv *team.Invitation) (*team.Invitation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invitation tx: %w", err)
	}
	defer tx.Rollback(ctx)
	row := tx.QueryRow(ctx, `
		INSERT INTO team_invitations (team_id, email, role, status, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+invitationCols, inv.TeamID, inv.Email, inv.Role, inv.Status,
		inv.TokenHash, inv.InvitedBy, inv.ExpiresAt)
	created, err := scanInvitation(row)
	if err != nil {
		return nil, err
	}
	admins, err := teamAdminIDs(ctx, tx, created.TeamID)
	if err != nil {
		return nil, err
	}
	if err := createTx(ctx, tx, admins, "team.invitation", "team_invitation", created.ID, map[string]any{
		"team_id": created.TeamID,
		"email":   created.Email,
		"role":    created.Role,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit invitation: %w", err)
	}
	return created, nil
}

func (r *teamRepo) GetInvitationByID(ctx context.Context, id uuid.UUID) (*team.Invitation, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+invitationCols+` FROM team_invitations WHERE id = $1`, id)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrInvitationNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *teamRepo) FindInvitationByHash(ctx context.Context, tokenHash string) (*team.Invitation, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+invitationCols+` FROM team_invitations WHERE token_hash = $1`, tokenHash)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrInvitationInvalid
		}
		return nil, err
	}
	return inv, nil
}

func (r *teamRepo) FindPendingInvitation(ctx context.Context, teamID uuid.UUID, email string) (*team.Invitation, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+invitationCols+`
		FROM team_invitations
		WHERE team_id = $1 AND lower(email) = lower($2) AND status = 'pending'
	`, teamID, email)
	inv, err := scanInvitation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrInvitationNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *teamRepo) ListPendingInvitations(ctx context.Context, teamID uuid.UUID) ([]*team.Invitation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.team_id, i.email, i.role, i.status, i.token_hash, i.invited_by,
		       i.expires_at, i.accepted_by, i.accepted_at, i.cancelled_by, i.cancelled_at, i.created_at,
		       u.display_name, t.name
		FROM team_invitations i
		JOIN users u ON u.id = i.invited_by
		JOIN teams t ON t.id = i.team_id
		WHERE i.team_id = $1 AND i.status = 'pending'
		ORDER BY i.created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*team.Invitation
	for rows.Next() {
		var inv team.Invitation
		if err := rows.Scan(
			&inv.ID, &inv.TeamID, &inv.Email, &inv.Role, &inv.Status, &inv.TokenHash, &inv.InvitedBy,
			&inv.ExpiresAt, &inv.AcceptedBy, &inv.AcceptedAt, &inv.CancelledBy, &inv.CancelledAt, &inv.CreatedAt,
			&inv.InviterName, &inv.TeamName,
		); err != nil {
			return nil, err
		}
		out = append(out, &inv)
	}
	return out, rows.Err()
}

func (r *teamRepo) UpdateInvitation(ctx context.Context, inv *team.Invitation) (*team.Invitation, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE team_invitations
		SET role = $1, status = $2, token_hash = $3, expires_at = $4,
		    accepted_by = $5, accepted_at = $6, cancelled_by = $7, cancelled_at = $8,
		    updated_at = NOW()
		WHERE id = $9
		RETURNING `+invitationCols, inv.Role, inv.Status, inv.TokenHash, inv.ExpiresAt,
		inv.AcceptedBy, inv.AcceptedAt, inv.CancelledBy, inv.CancelledAt, inv.ID)
	return scanInvitation(row)
}

func (r *teamRepo) AcceptInvitation(ctx context.Context, inv *team.Invitation, userID uuid.UUID) (*team.TeamMember, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Upsert an active membership. The unique (team_id, user_id) constraint lets a
	// previously removed/left member be reactivated.
	row := tx.QueryRow(ctx, `
		INSERT INTO team_members (team_id, user_id, role, status, invited_by, joined_at)
		VALUES ($1, $2, $3, 'active', $4, NOW())
		ON CONFLICT (team_id, user_id) DO UPDATE
		    SET status = 'active', role = EXCLUDED.role,
		        invited_by = EXCLUDED.invited_by, joined_at = NOW()
		RETURNING id, team_id, user_id, role, status,
		          invited_by, request_message, resolved_by, resolved_at, joined_at, created_at
	`, inv.TeamID, userID, inv.Role, inv.InvitedBy)
	m, err := scanMemberCore(row)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE team_invitations
		SET status = 'accepted', accepted_by = $1, accepted_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`, userID, inv.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

// ── Scanners ─────────────────────────────────────────────────────────────────

func scanTeam(row pgx.Row) (*team.Team, error) {
	var t team.Team
	err := row.Scan(
		&t.ID, &t.Name, &t.Description, &t.Currency, &t.IsPublic,
		&t.OwnerID, &t.CreatedBy, &t.CreatedAt, &t.DeletedAt, &t.DeletedBy,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTeams(rows pgx.Rows) ([]*team.Team, error) {
	var teams []*team.Team
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// scanMember scans a row that includes user join fields.
func scanMember(row pgx.Row) (*team.TeamMember, error) {
	var m team.TeamMember
	err := row.Scan(
		&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.Status,
		&m.InvitedBy, &m.RequestMessage, &m.ResolvedBy, &m.ResolvedAt,
		&m.JoinedAt, &m.CreatedAt,
		&m.UserDisplayName, &m.UserAvatarURL, &m.UserIdentityType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrNotMember
		}
		return nil, err
	}
	return &m, nil
}

// scanMemberCore scans a row that does NOT include user join fields (after writes).
func scanMemberCore(row pgx.Row) (*team.TeamMember, error) {
	var m team.TeamMember
	err := row.Scan(
		&m.ID, &m.TeamID, &m.UserID, &m.Role, &m.Status,
		&m.InvitedBy, &m.RequestMessage, &m.ResolvedBy, &m.ResolvedAt,
		&m.JoinedAt, &m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, team.ErrNotMember
		}
		return nil, err
	}
	return &m, nil
}

func scanMembers(rows pgx.Rows) ([]*team.TeamMember, error) {
	var members []*team.TeamMember
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func teamAdminIDs(ctx context.Context, tx pgx.Tx, teamID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
		SELECT user_id
		FROM team_members
		WHERE team_id = $1 AND status = 'active' AND role IN ('owner', 'admin')
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func scanInviteLink(row pgx.Row) (*team.InviteLink, error) {
	var l team.InviteLink
	err := row.Scan(
		&l.ID, &l.TeamID, &l.CreatedBy, &l.TokenHash,
		&l.MaxUses, &l.UseCount, &l.ExpiresAt, &l.RevokedAt, &l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func scanInvitation(row pgx.Row) (*team.Invitation, error) {
	var inv team.Invitation
	err := row.Scan(
		&inv.ID, &inv.TeamID, &inv.Email, &inv.Role, &inv.Status, &inv.TokenHash, &inv.InvitedBy,
		&inv.ExpiresAt, &inv.AcceptedBy, &inv.AcceptedAt, &inv.CancelledBy, &inv.CancelledAt, &inv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}
