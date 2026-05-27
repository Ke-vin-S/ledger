# Data Model Reference

Full schema is documented in @docs/data-model.md.

## Key invariants to enforce in code

**Split integrity**
- `SUM(expense_splits.share_amount WHERE expense_id = X AND version = current)` MUST equal `expenses.amount`
- Enforce before committing. Reject with 422 `INVALID_SPLIT_SUM`.

**Settlement ceiling**
- `SUM(settlements.amount WHERE expense_id = X AND payer_id = Y AND status != 'disputed')` MUST NOT exceed `expense_splits.share_amount` for that user.
- Reject with 409 `SETTLEMENT_EXCEEDS_DEBT`.

**Correction flow**
1. Increment `expenses.version`
2. Insert snapshot into `expense_versions`
3. Insert new `expense_splits` rows at the new version (do NOT update old rows)
4. Detect settlements against old split amounts — surface discrepancies in response `meta.discrepancies[]`
5. Write audit log entry with `action = 'expense.corrected'`, `before` and `after` snapshots

**Anonymous claim (must be a single transaction)**
1. Set `users.claimed_by` and `users.claimed_at`
2. Reassign all `expenses.created_by`, `expenses.paid_by` where value = anon user ID
3. Reassign all `expense_splits.user_id`
4. Reassign all `settlements.payer_id`, `settlements.payee_id`
5. Reassign all `team_members.user_id`
6. Mark `claim_tokens.used_at`
7. Write audit log: `action = 'user.claimed'`

**Soft deletes only**
- `expenses`: set `is_void = true`, `void_reason`, `voided_by`, `voided_at`
- `teams`: set `deleted_at`, `deleted_by`
- `audit_log`: never delete, never update

## Debt balance derivation

Debt is never stored as a static field. Always derived:

```sql
SELECT
  es.user_id        AS debtor_id,
  e.paid_by         AS creditor_id,
  es.share_amount   AS total_share,
  COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS total_settled,
  es.share_amount - COALESCE(SUM(s.amount) FILTER (WHERE s.status = 'confirmed'), 0) AS balance
FROM expense_splits es
JOIN expenses e ON e.id = es.expense_id
LEFT JOIN settlements s ON s.expense_id = es.expense_id AND s.payer_id = es.user_id
WHERE es.version = e.version
  AND e.is_void = false
  AND es.user_id != e.paid_by
GROUP BY es.user_id, e.paid_by, es.share_amount
```

Materialise this as a view `debt_balances`. Refresh on: expense_splits insert, settlement status change.

## Money

All amounts: `INT8` (PostgreSQL `bigint`) in minor currency units.
- LKR 4,500.00 → stored as `450000`
- Go type: `int64`
- API: JSON integer (no quotes, no decimal point)
- NEVER use `NUMERIC`, `DECIMAL`, or `FLOAT` for amounts.
