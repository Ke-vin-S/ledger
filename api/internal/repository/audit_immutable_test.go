package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Ke-vin-S/ledger/api/internal/audit"
)

// TestAuditLog_AppendOnly verifies the database itself blocks UPDATE and DELETE
// on audit_log (enforced by triggers in the schema), guaranteeing the
// append-only invariant regardless of application code.
func TestAuditLog_AppendOnly(t *testing.T) {
	pool := requireDB(t)
	truncateAll(t, pool)
	ctx := context.Background()

	actor := seedUser(t, pool, "Actor")
	logger := audit.NewLogger(pool)
	entityID := uuid.New()
	if err := logger.Log(ctx, audit.Entry{
		Action:     audit.ActionExpenseCreated,
		ActorID:    &actor,
		EntityType: "expense",
		EntityID:   entityID,
	}); err != nil {
		t.Fatalf("write audit entry: %v", err)
	}

	// UPDATE must be rejected by the trigger.
	if _, err := pool.Exec(ctx, `UPDATE audit_log SET entity_type = 'tampered' WHERE entity_id = $1`, entityID); err == nil {
		t.Error("expected UPDATE on audit_log to be blocked, but it succeeded")
	}

	// DELETE must be rejected by the trigger.
	if _, err := pool.Exec(ctx, `DELETE FROM audit_log WHERE entity_id = $1`, entityID); err == nil {
		t.Error("expected DELETE on audit_log to be blocked, but it succeeded")
	}

	// The row must still be present and intact.
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE entity_id = $1 AND entity_type = 'expense'`, entityID).Scan(&count); err != nil {
		t.Fatalf("recount: %v", err)
	}
	if count != 1 {
		t.Errorf("audit row count = %d, want 1 (immutable)", count)
	}
}
