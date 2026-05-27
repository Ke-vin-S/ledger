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
	row := r.pool.QueryRow(ctx, `
		INSERT INTO team_members (team_id, user_id, role, status, invited_by, request_message, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, team_id, user_id, role, status,
		          invited_by, request_message, resolved_by, resolved_at, joined_at, created_at
	`, m.TeamID, m.UserID, m.Role, m.Status, m.InvitedBy, m.RequestMessage, m.JoinedAt)
	return scanMemberCore(row)
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

func (r *teamRepo) RevokeInviteLink(ctx context.Context, linkID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE invite_links SET revoked_at = NOW() WHERE id = $1`, linkID)
	return err
}

func (r *teamRepo) IncrementInviteLinkUse(ctx context.Context, linkID uuid.UUID, _ *time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE invite_links SET use_count = use_count + 1 WHERE id = $1`, linkID)
	return err
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
