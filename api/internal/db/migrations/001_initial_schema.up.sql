-- ENUMs
CREATE TYPE identity_type AS ENUM ('registered', 'anonymous');
CREATE TYPE team_role AS ENUM ('owner', 'admin', 'member');
CREATE TYPE membership_status AS ENUM ('active', 'invited', 'requested', 'rejected', 'removed', 'left');
CREATE TYPE expense_scope AS ENUM ('personal', 'team', 'direct');
CREATE TYPE split_method AS ENUM ('equal', 'exact', 'percentage', 'shares');
CREATE TYPE settlement_status AS ENUM ('pending_confirmation', 'confirmed', 'disputed');
CREATE TYPE settlement_method AS ENUM ('cash', 'bank_transfer', 'upi', 'card', 'other');
CREATE TYPE debt_status AS ENUM ('outstanding', 'partially_repaid', 'settled');
CREATE TYPE flag_status AS ENUM ('open', 'resolved');
CREATE TYPE audit_action AS ENUM (
    'user.created', 'user.updated', 'user.claimed',
    'team.created', 'team.updated', 'team.deleted',
    'member.invited', 'member.requested', 'member.approved', 'member.rejected', 'member.removed', 'member.left',
    'expense.created', 'expense.corrected', 'expense.voided',
    'split.created', 'split.corrected',
    'settlement.created', 'settlement.confirmed', 'settlement.disputed',
    'flag.opened', 'flag.resolved',
    'loan.acknowledged', 'loan.disputed'
);

-- Users & Auth
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_type   identity_type NOT NULL DEFAULT 'registered',
    display_name    TEXT NOT NULL,
    email           TEXT UNIQUE,
    password_hash   TEXT,
    avatar_url      TEXT,
    currency_pref   CHAR(3) NOT NULL DEFAULT 'LKR',
    timezone        TEXT NOT NULL DEFAULT 'Asia/Colombo',
    claimed_by      UUID REFERENCES users(id),
    claimed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID REFERENCES users(id)
);

CREATE TABLE oauth_accounts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    provider     TEXT NOT NULL,
    provider_uid TEXT NOT NULL,
    email        TEXT,
    linked_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_uid)
);

CREATE TABLE claim_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    anon_user_id UUID NOT NULL REFERENCES users(id),
    created_by   UUID NOT NULL REFERENCES users(id),
    token_hash   TEXT NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    used_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Teams & Membership
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    currency    CHAR(3) NOT NULL DEFAULT 'LKR',
    is_public   BOOLEAN NOT NULL DEFAULT FALSE,
    owner_id    UUID NOT NULL REFERENCES users(id),
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID REFERENCES users(id)
);

CREATE TABLE team_members (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id         UUID NOT NULL REFERENCES teams(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    role            team_role NOT NULL DEFAULT 'member',
    status          membership_status NOT NULL,
    invited_by      UUID REFERENCES users(id),
    request_message TEXT,
    resolved_by     UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    joined_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (team_id, user_id)
);

CREATE TABLE invite_links (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id    UUID NOT NULL REFERENCES teams(id),
    created_by UUID NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL UNIQUE,
    max_uses   INT,
    use_count  INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Categories
CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    icon       TEXT,
    team_id    UUID REFERENCES teams(id),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Expenses
CREATE TABLE expenses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope        expense_scope NOT NULL,
    team_id      UUID REFERENCES teams(id),
    title        TEXT NOT NULL,
    amount       BIGINT NOT NULL CHECK (amount > 0),
    currency     CHAR(3) NOT NULL DEFAULT 'LKR',
    category_id  UUID REFERENCES categories(id),
    paid_by      UUID NOT NULL REFERENCES users(id),
    expense_date DATE NOT NULL,
    split_method split_method,
    receipt_url  TEXT,
    note         TEXT,
    version      INT NOT NULL DEFAULT 1,
    is_void      BOOLEAN NOT NULL DEFAULT FALSE,
    void_reason  TEXT,
    voided_by    UUID REFERENCES users(id),
    voided_at    TIMESTAMPTZ,
    created_by   UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE expense_versions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id        UUID NOT NULL REFERENCES expenses(id),
    version           INT NOT NULL,
    snapshot          JSONB NOT NULL,
    corrected_by      UUID NOT NULL REFERENCES users(id),
    correction_reason TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (expense_id, version)
);

CREATE TABLE expense_splits (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id   UUID NOT NULL REFERENCES expenses(id),
    user_id      UUID NOT NULL REFERENCES users(id),
    share_amount BIGINT NOT NULL CHECK (share_amount >= 0),
    share_units  NUMERIC,
    version      INT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Settlements
CREATE TABLE settlements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id   UUID NOT NULL REFERENCES expenses(id),
    payer_id     UUID NOT NULL REFERENCES users(id),
    payee_id     UUID NOT NULL REFERENCES users(id),
    amount       BIGINT NOT NULL CHECK (amount > 0),
    method       settlement_method NOT NULL,
    method_note  TEXT,
    status       settlement_status NOT NULL DEFAULT 'pending_confirmation',
    recorded_by  UUID NOT NULL REFERENCES users(id),
    confirmed_by UUID REFERENCES users(id),
    confirmed_at TIMESTAMPTZ,
    disputed_by  UUID REFERENCES users(id),
    disputed_at  TIMESTAMPTZ,
    dispute_reason TEXT,
    settled_on   DATE NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Flags
CREATE TABLE expense_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id      UUID NOT NULL REFERENCES expenses(id),
    raised_by       UUID NOT NULL REFERENCES users(id),
    reason          TEXT NOT NULL,
    status          flag_status NOT NULL DEFAULT 'open',
    resolved_by     UUID REFERENCES users(id),
    resolution_note TEXT,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Notifications
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    type        TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notification_prefs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) UNIQUE,
    email_enabled  BOOLEAN NOT NULL DEFAULT TRUE,
    digest_mode    BOOLEAN NOT NULL DEFAULT FALSE,
    disabled_types TEXT[] NOT NULL DEFAULT '{}',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit Log (append-only)
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action      audit_action NOT NULL,
    actor_id    UUID REFERENCES users(id),
    team_id     UUID REFERENCES teams(id),
    entity_type TEXT NOT NULL,
    entity_id   UUID NOT NULL,
    before      JSONB,
    after       JSONB,
    meta        JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Prevent UPDATE and DELETE on audit_log
CREATE OR REPLACE FUNCTION audit_log_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log rows are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

CREATE TRIGGER audit_log_no_delete
    BEFORE DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_immutable();

-- Derived view: debt balances per expense split
CREATE VIEW debt_balances AS
SELECT
    es.expense_id,
    es.user_id                  AS debtor_id,
    e.paid_by                   AS creditor_id,
    e.team_id,
    es.share_amount             AS total_share,
    COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS total_settled,
    es.share_amount - COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS balance,
    CASE
        WHEN es.share_amount - COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) <= 0 THEN 'settled'::debt_status
        WHEN COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) > 0 THEN 'partially_repaid'::debt_status
        ELSE 'outstanding'::debt_status
    END AS debt_status
FROM expense_splits es
JOIN expenses e ON e.id = es.expense_id
LEFT JOIN settlements s ON s.expense_id = es.expense_id AND s.payer_id = es.user_id
WHERE es.version = e.version
  AND e.is_void = FALSE
  AND es.user_id != e.paid_by
GROUP BY es.expense_id, es.user_id, e.paid_by, e.team_id, es.share_amount;

-- Derived view: net balance per user pair per team
CREATE VIEW team_net_balances AS
SELECT
    team_id,
    LEAST(debtor_id, creditor_id)    AS user_a,
    GREATEST(debtor_id, creditor_id) AS user_b,
    SUM(CASE WHEN debtor_id < creditor_id THEN balance ELSE -balance END) AS net_amount
FROM debt_balances
GROUP BY team_id, LEAST(debtor_id, creditor_id), GREATEST(debtor_id, creditor_id);

-- Derived view: net balance per user pair across all teams
CREATE VIEW user_net_balances AS
SELECT
    debtor_id   AS user_id,
    creditor_id AS counterparty_id,
    SUM(balance) AS net_amount
FROM debt_balances
GROUP BY debtor_id, creditor_id;

-- Indexes
CREATE INDEX idx_team_members_team_id         ON team_members (team_id);
CREATE INDEX idx_team_members_user_id         ON team_members (user_id);
CREATE INDEX idx_expenses_team_id_date        ON expenses (team_id, expense_date DESC) WHERE team_id IS NOT NULL;
CREATE INDEX idx_expenses_paid_by             ON expenses (paid_by);
CREATE INDEX idx_expenses_created_by          ON expenses (created_by);
CREATE INDEX idx_expense_splits_expense_id    ON expense_splits (expense_id, version);
CREATE INDEX idx_expense_splits_user_id       ON expense_splits (user_id);
CREATE INDEX idx_settlements_expense_id       ON settlements (expense_id);
CREATE INDEX idx_settlements_payer_payee      ON settlements (payer_id, payee_id);
CREATE INDEX idx_settlements_status           ON settlements (status);
CREATE INDEX idx_audit_log_entity             ON audit_log (entity_type, entity_id);
CREATE INDEX idx_audit_log_actor              ON audit_log (actor_id, created_at DESC);
CREATE INDEX idx_audit_log_team               ON audit_log (team_id, created_at DESC) WHERE team_id IS NOT NULL;
CREATE INDEX idx_notifications_user           ON notifications (user_id, is_read, created_at DESC);

-- Seed global categories
INSERT INTO categories (id, name, icon) VALUES
    (gen_random_uuid(), 'Food & Drinks', '🍔'),
    (gen_random_uuid(), 'Transport', '🚗'),
    (gen_random_uuid(), 'Accommodation', '🏨'),
    (gen_random_uuid(), 'Entertainment', '🎬'),
    (gen_random_uuid(), 'Shopping', '🛍️'),
    (gen_random_uuid(), 'Health', '🏥'),
    (gen_random_uuid(), 'Utilities', '💡'),
    (gen_random_uuid(), 'Other', '📦');
