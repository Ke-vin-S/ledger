DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM settlements s
        JOIN expenses e ON e.id = s.expense_id
        WHERE s.payer_id = s.payee_id OR s.payee_id <> e.paid_by
    ) THEN
        RAISE EXCEPTION 'cannot enforce settlement integrity: existing settlements pay a non-creditor or themselves';
    END IF;
END $$;

CREATE OR REPLACE VIEW debt_balances AS
SELECT
    es.expense_id,
    es.user_id AS debtor_id,
    e.paid_by AS creditor_id,
    e.team_id,
    es.share_amount AS total_share,
    COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS total_settled,
    es.share_amount - COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS balance,
    CASE
        WHEN es.share_amount - COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) <= 0 THEN 'settled'::debt_status
        WHEN COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) > 0 THEN 'partially_repaid'::debt_status
        ELSE 'outstanding'::debt_status
    END AS debt_status
FROM expense_splits es
JOIN expenses e ON e.id = es.expense_id
LEFT JOIN settlements s
  ON s.expense_id = es.expense_id
 AND s.payer_id = es.user_id
 AND s.payee_id = e.paid_by
WHERE es.version = e.version
  AND e.is_void = FALSE
  AND es.user_id != e.paid_by
GROUP BY es.expense_id, es.user_id, e.paid_by, e.team_id, es.share_amount;

CREATE INDEX idx_settlements_balance
    ON settlements(expense_id, payer_id, payee_id, status);

ALTER TABLE settlements
    ADD CONSTRAINT settlements_payer_ne_payee CHECK (payer_id <> payee_id);
