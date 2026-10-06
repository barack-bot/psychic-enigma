package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MarkLedgerRepository records immutable mark transitions for audit.
type MarkLedgerRepository struct{ db *pgxpool.Pool }

func NewMarkLedgerRepository(db *pgxpool.Pool) *MarkLedgerRepository {
	return &MarkLedgerRepository{db: db}
}

func (r *MarkLedgerRepository) Record(ctx context.Context, markID int64, fromStatus, toStatus string, actorID int64, reason string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, details)
		 VALUES ('staff', $1, 'mark_transition', 'mark', $2, jsonb_build_object('from', $3, 'to', $4, 'reason', $5))`,
		actorID, markID, fromStatus, toStatus, reason)
	return err
}

func (r *MarkLedgerRepository) ListByMark(ctx context.Context, markID int64) ([]map[string]any, error) {
	rows, err := r.db.Query(ctx,
		`SELECT actor_id, action, details, created_at FROM audit_logs WHERE target_type = 'mark' AND target_id = $1 ORDER BY created_at`, markID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []map[string]any
	for rows.Next() {
		var actorID int64
		var action string
		var details []byte
		var createdAt any
		if err := rows.Scan(&actorID, &action, &details, &createdAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"actor_id": actorID, "action": action, "details": string(details), "created_at": createdAt})
	}
	return result, rows.Err()
}