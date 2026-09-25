ALTER TABLE settlements DROP CONSTRAINT IF EXISTS settlements_payer_ne_payee;
DROP INDEX IF EXISTS idx_settlements_balance;

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
LEFT JOIN settlements s ON s.expense_id = es.expense_id AND s.payer_id = es.user_id
WHERE es.version = e.version
  AND e.is_void = FALSE
  AND es.user_id != e.paid_by
GROUP BY es.expense_id, es.user_id, e.paid_by, e.team_id, es.share_amount;
