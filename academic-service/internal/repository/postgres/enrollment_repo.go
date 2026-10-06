package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
)

type EnrollmentRepository struct{ db *pgxpool.Pool }

func NewEnrollmentRepository(db *pgxpool.Pool) *EnrollmentRepository {
	return &EnrollmentRepository{db: db}
}

func (r *EnrollmentRepository) ListByStudent(ctx context.Context, studentID int64) ([]domain.Enrollment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, student_id, academic_term_id, class_offering_id, status, COALESCE(reported_at::text,''), COALESCE(deferred_reason,'') FROM student_term_enrollment WHERE student_id = $1 ORDER BY id DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Enrollment
	for rows.Next() {
		var e domain.Enrollment
		if err := rows.Scan(&e.ID, &e.StudentID, &e.AcademicTermID, &e.ClassOfferingID, &e.Status, &e.ReportedAt, &e.DeferredReason); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *EnrollmentRepository) Upsert(ctx context.Context, e domain.Enrollment) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO student_term_enrollment (student_id, academic_term_id, class_offering_id, status) VALUES ($1,$2,$3,$4)
		 ON CONFLICT (student_id, academic_term_id) DO UPDATE SET class_offering_id = EXCLUDED.class_offering_id, status = EXCLUDED.status, version = student_term_enrollment.version + 1
		 RETURNING id`,
		e.StudentID, e.AcademicTermID, e.ClassOfferingID, e.Status).Scan(&id)
	return id, err
}

func (r *EnrollmentRepository) UpdateStatus(ctx context.Context, enrollmentID int64, status, reason string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE student_term_enrollment SET status = $1, deferred_reason = NULLIF($2,''), reported_at = CASE WHEN $1 = 'ACTIVE' THEN CURRENT_TIMESTAMP ELSE reported_at END, version = version + 1 WHERE id = $3`,
		status, reason, enrollmentID)
	return err
}

func (r *EnrollmentRepository) Status(ctx context.Context, enrollmentID int64) (string, error) { var status string; err := r.db.QueryRow(ctx, `SELECT status FROM student_term_enrollment WHERE id=$1`, enrollmentID).Scan(&status); return status, err }