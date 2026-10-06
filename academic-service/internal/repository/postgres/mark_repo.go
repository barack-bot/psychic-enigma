package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
)

type MarkRepository struct{ db *pgxpool.Pool }

func NewMarkRepository(db *pgxpool.Pool) *MarkRepository {
	return &MarkRepository{db: db}
}

func (r *MarkRepository) ListByStatus(ctx context.Context, status string) ([]domain.Mark, error) {
	query := `SELECT id, assessment_id, student_id, score, status, submitted_by, approved_by, COALESCE(rejection_reason,'') FROM marks`
	args := []any{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Mark
	for rows.Next() {
		var m domain.Mark
		if err := rows.Scan(&m.ID, &m.AssessmentID, &m.StudentID, &m.Score, &m.Status, &m.SubmittedBy, &m.ApprovedBy, &m.RejectionReason); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *MarkRepository) Upsert(ctx context.Context, m domain.Mark) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO marks (assessment_id, student_id, score, submitted_by) VALUES ($1,$2,$3,$4)
			 ON CONFLICT (assessment_id, student_id) DO UPDATE SET score = EXCLUDED.score, submitted_by = EXCLUDED.submitted_by, status = 'DRAFT', version = marks.version + 1, approved_by = NULL, approved_at = NULL, rejection_reason = NULL
			 WHERE marks.status IN ('DRAFT', 'REJECTED')
		 RETURNING id`,
		m.AssessmentID, m.StudentID, m.Score, m.SubmittedBy).Scan(&id)
	return id, err
}

func (r *MarkRepository) Transition(ctx context.Context, markID int64, from, to string, actorID int64, reason string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `UPDATE marks SET status = $1, rejection_reason = NULLIF($2,''), version = version + 1 WHERE id = $3 AND status = $4`
	args := []any{to, reason, markID, from}
	switch to {
	case "SUBMITTED":
		query += ` AND EXISTS (SELECT 1 FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id WHERE a.id = marks.assessment_id AND cs.teacher_id = $5)`
		args = append(args, actorID)
	case "APPROVED", "REJECTED":
		query += ` AND EXISTS (SELECT 1 FROM assessments a JOIN class_subjects cs ON cs.id = a.class_subject_id JOIN subjects s ON s.id = cs.subject_id JOIN staff_users u ON u.id = $5 WHERE a.id = marks.assessment_id AND u.role = 'hod' AND (s.department_id IS NULL OR u.department_id = s.department_id))`
		args = append(args, actorID)
	default:
		return domain.ErrInvalidStatus
	}
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrUnauthorized
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, details)
		 VALUES ('staff', $1, 'mark_transition', 'mark', $2, jsonb_build_object('from', $3, 'to', $4, 'reason', $5))`,
		actorID, markID, from, to, reason)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}