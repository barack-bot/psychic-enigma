package service

import (
	"context"

	"school-platform/academic-service/internal/domain"
)

type RosterStore interface {
	ListClassSubjects(context.Context, int64) ([]domain.ClassSubject, error)
	ListByStudent(context.Context, int64) ([]domain.Enrollment, error)
}

// RosterService assembles class lists and subject rosters for a teacher.
type RosterService struct{ store RosterStore }

func NewRosterService(store RosterStore) *RosterService {
	return &RosterService{store: store}
}

// StudentRoster returns a student's term enrollments.
func (s *RosterService) StudentRoster(ctx context.Context, studentID int64) ([]domain.Enrollment, error) {
	return s.store.ListByStudent(ctx, studentID)
}