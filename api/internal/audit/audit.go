package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Action string

const (
	ActionUserCreated       Action = "user.created"
	ActionUserUpdated       Action = "user.updated"
	ActionUserClaimed       Action = "user.claimed"
	ActionTeamCreated       Action = "team.created"
	ActionTeamUpdated       Action = "team.updated"
	ActionTeamDeleted       Action = "team.deleted"
	ActionMemberInvited     Action = "member.invited"
	ActionMemberRequested   Action = "member.requested"
	ActionMemberApproved    Action = "member.approved"
	ActionMemberRejected    Action = "member.rejected"
	ActionMemberRemoved     Action = "member.removed"
	ActionMemberLeft        Action = "member.left"
	ActionExpenseCreated    Action = "expense.created"
	ActionExpenseCorrected  Action = "expense.corrected"
	ActionExpenseVoided     Action = "expense.voided"
	ActionSplitCreated      Action = "split.created"
	ActionSplitCorrected    Action = "split.corrected"
	ActionSettlementCreated Action = "settlement.created"
	ActionSettlementConfirmed Action = "settlement.confirmed"
	ActionSettlementDisputed  Action = "settlement.disputed"
	ActionFlagOpened        Action = "flag.opened"
	ActionFlagResolved      Action = "flag.resolved"
	ActionLoanAcknowledged  Action = "loan.acknowledged"
	ActionLoanDisputed      Action = "loan.disputed"
)

type Entry struct {
	Action     Action
	ActorID    *uuid.UUID
	TeamID     *uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	Before     any
	After      any
	Meta       map[string]any
}

type Logger interface {
	Log(ctx context.Context, e Entry) error
}

type postgresLogger struct {
	pool *pgxpool.Pool
}

func NewLogger(pool *pgxpool.Pool) Logger {
	return &postgresLogger{pool: pool}
}

func (l *postgresLogger) Log(ctx context.Context, e Entry) error {
	before, err := marshalJSON(e.Before)
	if err != nil {
		return fmt.Errorf("marshal before: %w", err)
	}
	after, err := marshalJSON(e.After)
	if err != nil {
		return fmt.Errorf("marshal after: %w", err)
	}
	meta, err := marshalJSON(e.Meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if meta == nil {
		meta = []byte("{}")
	}

	_, err = l.pool.Exec(ctx, `
		INSERT INTO audit_log (action, actor_id, team_id, entity_type, entity_id, before, after, meta)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, e.Action, e.ActorID, e.TeamID, e.EntityType, e.EntityID, before, after, meta)
	return err
}

func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

type nopLogger struct{}

func NopLogger() Logger { return &nopLogger{} }

func (*nopLogger) Log(_ context.Context, _ Entry) error { return nil }
