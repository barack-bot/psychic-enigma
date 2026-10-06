package service

import (
	"context"
	"errors"

	"school-platform/intake-service/internal/domain"
)

type IntakeStore interface {
	Create(context.Context, domain.Student) (int64, error)
}

// IntakeService handles admissions-side student creation.
type IntakeService struct{ store IntakeStore }

func NewIntakeService(store IntakeStore) *IntakeService {
	return &IntakeService{store: store}
}

func (s *IntakeService) RegisterStudent(ctx context.Context, student domain.Student) (int64, error) {
	if student.Name == "" || student.IDNumber == "" || student.DateOfBirth == "" {
		return 0, errors.New("name, id_number and date_of_birth are required")
	}
	if student.Status == "" {
		student.Status = "ACTIVE"
	}
	return s.store.Create(ctx, student)
}