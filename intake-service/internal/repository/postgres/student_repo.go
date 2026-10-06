package postgres

import (
	"context"
	"database/sql"
	"errors"

	"school-platform/intake-service/internal/domain"
)

type StudentRepository struct{ db *sql.DB }

func NewStudentRepository(db *sql.DB) *StudentRepository {
	return &StudentRepository{db: db}
}

func (r *StudentRepository) Create(ctx context.Context, s domain.Student) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO students (name, id_number, email, date_of_birth, phone_number, admission_number, status) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),$7) RETURNING student_id`,
		s.Name, s.IDNumber, s.Email, s.DateOfBirth, s.PhoneNumber, s.AdmissionNumber, s.Status).Scan(&id)
	return id, err
}

func (r *StudentRepository) FindByID(ctx context.Context, id int64) (domain.Student, error) {
	var s domain.Student
	err := r.db.QueryRowContext(ctx,
		`SELECT student_id, name, id_number, COALESCE(email,''), date_of_birth::text, COALESCE(phone_number,''), COALESCE(admission_number,''), status FROM students WHERE student_id = $1 AND deleted_at IS NULL`, id).
		Scan(&s.ID, &s.Name, &s.IDNumber, &s.Email, &s.DateOfBirth, &s.PhoneNumber, &s.AdmissionNumber, &s.Status)
	return s, err
}

func (r *StudentRepository) List(ctx context.Context, includeDeleted bool) ([]domain.Student, error) {
	query := `SELECT student_id, name, id_number, COALESCE(email,''), date_of_birth::text, COALESCE(phone_number,''), COALESCE(admission_number,''), status FROM students WHERE deleted_at IS NULL ORDER BY student_id`
	if includeDeleted {
		query = `SELECT student_id, name, id_number, COALESCE(email,''), date_of_birth::text, COALESCE(phone_number,''), COALESCE(admission_number,''), status FROM students ORDER BY student_id`
	}
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Student
	for rows.Next() {
		var s domain.Student
		if err := rows.Scan(&s.ID, &s.Name, &s.IDNumber, &s.Email, &s.DateOfBirth, &s.PhoneNumber, &s.AdmissionNumber, &s.Status); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *StudentRepository) SoftDelete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE students SET deleted_at = CURRENT_TIMESTAMP, status = 'WITHDRAWN' WHERE student_id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return errors.New("student not found")
	}
	return nil
}