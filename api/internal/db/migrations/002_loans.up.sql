CREATE TYPE loan_direction AS ENUM ('lent', 'borrowed');
CREATE TYPE loan_status AS ENUM ('outstanding', 'partially_repaid', 'settled', 'disputed');

CREATE TABLE loans (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id),
    direction         loan_direction NOT NULL,
    amount            BIGINT NOT NULL CHECK (amount > 0),
    currency          CHAR(3) NOT NULL,
    counterparty_id   UUID REFERENCES users(id),
    counterparty_name TEXT NOT NULL,
    note              TEXT,
    status            loan_status NOT NULL DEFAULT 'outstanding',
    acknowledged_at   TIMESTAMPTZ,
    loan_date         DATE NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE loan_repayments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loan_id     UUID NOT NULL REFERENCES loans(id),
    amount      BIGINT NOT NULL CHECK (amount > 0),
    note        TEXT,
    repaid_at   DATE NOT NULL,
    recorded_by UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
