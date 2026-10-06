package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
)

type ClassRepository struct{ db *pgxpool.Pool }

func NewClassRepository(db *pgxpool.Pool) *ClassRepository {
	return &ClassRepository{db: db}
}

func (r *ClassRepository) ListClassOfferings(ctx context.Context, termID *int64) ([]domain.ClassOffering, error) {
	query := `SELECT id, course_id, academic_term_id, name, class_teacher_id, capacity FROM class_offerings`
	args := []any{}
	if termID != nil {
		query += ` WHERE academic_term_id = $1`
		args = append(args, *termID)
	}
	query += ` ORDER BY id DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.ClassOffering
	for rows.Next() {
		var c domain.ClassOffering
		if err := rows.Scan(&c.ID, &c.CourseID, &c.AcademicTermID, &c.Name, &c.ClassTeacherID, &c.Capacity); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *ClassRepository) CreateClassOffering(ctx context.Context, c domain.ClassOffering) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO class_offerings (course_id, academic_term_id, name, class_teacher_id, capacity) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		c.CourseID, c.AcademicTermID, c.Name, c.ClassTeacherID, c.Capacity).Scan(&id)
	return id, err
}

func (r *ClassRepository) ListClassSubjects(ctx context.Context, classOfferingID int64) ([]domain.ClassSubject, error) {
	rows, err := r.db.Query(ctx, `SELECT id, class_offering_id, subject_id, teacher_id FROM class_subjects WHERE class_offering_id = $1`, classOfferingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.ClassSubject
	for rows.Next() {
		var cs domain.ClassSubject
		if err := rows.Scan(&cs.ID, &cs.ClassOfferingID, &cs.SubjectID, &cs.TeacherID); err != nil {
			return nil, err
		}
		result = append(result, cs)
	}
	return result, rows.Err()
}

func (r *ClassRepository) ListByStudent(ctx context.Context, studentID int64) ([]domain.Enrollment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, student_id, academic_term_id, class_offering_id, status, COALESCE(reported_at::text,''), COALESCE(deferred_reason,'') FROM student_term_enrollment WHERE student_id = $1 ORDER BY id DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Enrollment
	for rows.Next() {
		var enrollment domain.Enrollment
		if err := rows.Scan(&enrollment.ID, &enrollment.StudentID, &enrollment.AcademicTermID, &enrollment.ClassOfferingID, &enrollment.Status, &enrollment.ReportedAt, &enrollment.DeferredReason); err != nil {
			return nil, err
		}
		result = append(result, enrollment)
	}
	return result, rows.Err()
}

func (r *ClassRepository) AssignTeacher(ctx context.Context, classSubjectID, teacherID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE class_subjects SET teacher_id = $1 WHERE id = $2`, teacherID, classSubjectID)
	return err
}