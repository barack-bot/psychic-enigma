package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
)

type TeacherRepository struct{ db *pgxpool.Pool }

func NewTeacherRepository(db *pgxpool.Pool) *TeacherRepository {
	return &TeacherRepository{db: db}
}

func (r *TeacherRepository) List(ctx context.Context) ([]domain.Teacher, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(employee_no,''), name, COALESCE(email,''), department_id, status FROM staff_users WHERE role IN ('teacher','hod') ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Teacher
	for rows.Next() {
		var t domain.Teacher
		if err := rows.Scan(&t.ID, &t.EmployeeNo, &t.Name, &t.Email, &t.DepartmentID, &t.Status); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *TeacherRepository) FindByID(ctx context.Context, id int64) (domain.Teacher, error) {
	var t domain.Teacher
	err := r.db.QueryRow(ctx,
		`SELECT id, COALESCE(employee_no,''), name, COALESCE(email,''), department_id, status FROM staff_users WHERE id = $1`, id).
		Scan(&t.ID, &t.EmployeeNo, &t.Name, &t.Email, &t.DepartmentID, &t.Status)
	return t, err
}

func (r *TeacherRepository) ListByDepartment(ctx context.Context, departmentID int64) ([]domain.Teacher, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(employee_no,''), name, COALESCE(email,''), department_id, status FROM staff_users WHERE department_id = $1 AND role IN ('teacher','hod') ORDER BY name`, departmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Teacher
	for rows.Next() {
		var t domain.Teacher
		if err := rows.Scan(&t.ID, &t.EmployeeNo, &t.Name, &t.Email, &t.DepartmentID, &t.Status); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}