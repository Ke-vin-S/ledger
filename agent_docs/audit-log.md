# Audit Log Reference

## Principle

The audit log is append-only. No UPDATE or DELETE is ever issued against `audit_log`. This is enforced at the application layer and should be enforced at the database layer with a trigger or RLS policy.

## Writing a log entry

Every domain service that performs a write operation must call the audit logger. The logger is injected as a dependency — never instantiated inside a service.

```go
type AuditLogger interface {
    Log(ctx context.Context, entry AuditEntry) error
}

type AuditEntry struct {
    Action     AuditAction
    ActorID    *uuid.UUID // nil for system events
    TeamID     *uuid.UUID // nil for user-level events
    EntityType string
    EntityID   uuid.UUID
    Before     any        // nil for creates
    After      any        // nil for deletes
    Meta       map[string]any
}
```

## Required log entries per action

| Operation | Action | Before | After | Notes |
|---|---|---|---|---|
| Create expense | `expense.created` | nil | expense row | |
| Correct expense | `expense.corrected` | old snapshot | new snapshot | Include `correction_reason` in Meta |
| Void expense | `expense.voided` | expense row | nil | Include `void_reason` in Meta |
| Create settlement | `settlement.created` | nil | settlement row | |
| Confirm settlement | `settlement.confirmed` | old settlement | updated settlement | |
| Dispute settlement | `settlement.disputed` | old settlement | updated settlement | Include `dispute_reason` in Meta |
| Raise flag | `flag.opened` | nil | flag row | |
| Resolve flag | `flag.resolved` | old flag | updated flag | Include `resolution_note` in Meta |
| Claim anonymous | `user.claimed` | anon user | registered user | Include both IDs in Meta |
| Approve join request | `member.approved` | old membership | updated membership | |
| Reject join request | `member.rejected` | old membership | updated membership | |
| Remove member | `member.removed` | membership row | nil | |

## Log entry for corrections with settlement discrepancies

When a correction changes a split that has confirmed settlements, include in `Meta`:

```json
{
  "correction_reason": "...",
  "discrepancies": [
    {
      "user_id": "...",
      "old_share": 120000,
      "new_share": 150000,
      "confirmed_settled": 120000
    }
  ]
}
```

These discrepancies are also returned in the API response `meta.discrepancies[]` so the caller can surface them in the UI.

## Do not log

- Read operations (GET requests) — no audit entries.
- Internal system operations with no user actor — if truly needed, set `ActorID = nil` and include context in `Meta`.
- Notification delivery — not a financial event.
