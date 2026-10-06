package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/academic-service/internal/domain"
)

type SubjectRepository struct{ db *pgxpool.Pool }

func NewSubjectRepository(db *pgxpool.Pool) *SubjectRepository {
	return &SubjectRepository{db: db}
}

func (r *SubjectRepository) List(ctx context.Context) ([]domain.Subject, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, COALESCE(code,''), department_id FROM subjects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Subject
	for rows.Next() {
		var s domain.Subject
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.DepartmentID); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *SubjectRepository) Create(ctx context.Context, s domain.Subject) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`INSERT INTO subjects (name, code, department_id) VALUES ($1, NULLIF($2,''), $3) RETURNING id`,
		s.Name, s.Code, s.DepartmentID).Scan(&id)
	return id, err
}

func (r *SubjectRepository) FindByID(ctx context.Context, id int64) (domain.Subject, error) {
	var s domain.Subject
	err := r.db.QueryRow(ctx, `SELECT id, name, COALESCE(code,''), department_id FROM subjects WHERE id = $1`, id).
		Scan(&s.ID, &s.Name, &s.Code, &s.DepartmentID)
	return s, err
}