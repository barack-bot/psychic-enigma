package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/service"
	"school-platform/shared/events"
)

type StudentProjectionStore struct{ db *pgxpool.Pool }

func NewStudentProjectionStore(db *pgxpool.Pool) *StudentProjectionStore {
	return &StudentProjectionStore{db: db}
}

func (s *StudentProjectionStore) ApplyStudentEvent(ctx context.Context, event events.Event, student events.Student) error {
	// The unified schema keeps the source-of-truth student rows in the canonical
	// students table; legacy projection tables have been removed.
	_ = ctx
	_ = event
	_ = student
	return nil
}

func (s *StudentProjectionStore) ListStudents(ctx context.Context) ([]service.AcademicStudent, error) {
	rows, err := s.db.Query(ctx, `SELECT student_id, admission_number, name, status, version FROM students`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []service.AcademicStudent
	for rows.Next() {
		var student service.AcademicStudent
		if err := rows.Scan(&student.ID, &student.AdmissionNo, &student.Name, &student.Status, &student.SourceVersion); err != nil {
			return nil, err
		}
		result = append(result, student)
	}
	return result, rows.Err()
}
