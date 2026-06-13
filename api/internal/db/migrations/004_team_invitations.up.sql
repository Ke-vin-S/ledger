-- Email-based team invitations. Unlike team_members (which requires a real
-- user_id) these are keyed by email, so a person can be invited before they
-- have an account. Status is pending until the invitee accepts via the token
-- link, at which point a team_members row is created.

CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'cancelled', 'expired');

CREATE TABLE team_invitations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id      UUID NOT NULL REFERENCES teams(id),
    email        TEXT NOT NULL,
    role         team_role NOT NULL DEFAULT 'member',
    status       invitation_status NOT NULL DEFAULT 'pending',
    token_hash   TEXT NOT NULL UNIQUE,
    invited_by   UUID NOT NULL REFERENCES users(id),
    expires_at   TIMESTAMPTZ NOT NULL,
    accepted_by  UUID REFERENCES users(id),
    accepted_at  TIMESTAMPTZ,
    cancelled_by UUID REFERENCES users(id),
    cancelled_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- At most one pending invitation per team + email (case-insensitive).
CREATE UNIQUE INDEX uq_team_invitations_pending
    ON team_invitations (team_id, lower(email))
    WHERE status = 'pending';

CREATE INDEX idx_team_invitations_team ON team_invitations (team_id);
