CREATE TABLE anonymous_users (
    anon_user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    created_by   UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_anonymous_users_created_by ON anonymous_users (created_by);
