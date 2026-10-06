package service

import (
	"context"
	"errors"

	"school-platform/academic-service/internal/domain"
)

type MarkSubmissionStore interface {
	Upsert(context.Context, domain.Mark) (int64, error)
	Transition(context.Context, int64, string, string, int64, string) error
}

type MarkSubmissionService struct{ store MarkSubmissionStore }

func NewMarkSubmissionService(store MarkSubmissionStore) *MarkSubmissionService {
	return &MarkSubmissionService{store: store}
}

// SaveDraft persists a mark as DRAFT (or resets a REJECTED mark back to DRAFT).
func (s *MarkSubmissionService) SaveDraft(ctx context.Context, m domain.Mark) (int64, error) {
	if m.AssessmentID <= 0 || m.StudentID <= 0 {
		return 0, errors.New("assessment_id and student_id are required")
	}
	if m.Score < 0 {
		return 0, domain.ErrInvalidScore
	}
	return s.store.Upsert(ctx, m)
}

// Submit transitions a DRAFT or REJECTED mark to SUBMITTED.
func (s *MarkSubmissionService) Submit(ctx context.Context, markID, teacherID int64) error {
	if markID <= 0 || teacherID <= 0 {
		return errors.New("mark_id and teacher_id are required")
	}
	return s.store.Transition(ctx, markID, "DRAFT", "SUBMITTED", teacherID, "")
}