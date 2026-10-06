package service

import (
	"context"
	"errors"

	"school-platform/academic-service/internal/domain"
)

type EnrollmentStore interface {
	ListByStudent(context.Context, int64) ([]domain.Enrollment, error)
	Upsert(context.Context, domain.Enrollment) (int64, error)
	UpdateStatus(context.Context, int64, string, string) error
	Status(context.Context, int64) (string, error)
}

type EnrollmentService struct{ store EnrollmentStore }

func NewEnrollmentService(store EnrollmentStore) *EnrollmentService {
	return &EnrollmentService{store: store}
}

func (s *EnrollmentService) ListForStudent(ctx context.Context, studentID int64) ([]domain.Enrollment, error) {
	return s.store.ListByStudent(ctx, studentID)
}

func (s *EnrollmentService) Enroll(ctx context.Context, e domain.Enrollment) (int64, error) {
	if e.Status == "" {
		e.Status = "ENROLLED"
	}
	if e.Status != "ENROLLED" && e.Status != "ACTIVE" { return 0, domain.ErrInvalidStatus }
	return s.store.Upsert(ctx, e)
}

func (s *EnrollmentService) Defer(ctx context.Context, enrollmentID int64, reason string) error {
	if reason == "" { return errors.New("deferred reason is required") }
	if err := s.validateTransition(ctx, enrollmentID, "DEFERRED"); err != nil { return err }
	return s.store.UpdateStatus(ctx, enrollmentID, "DEFERRED", reason)
}

func (s *EnrollmentService) Withdraw(ctx context.Context, enrollmentID int64) error {
	if err := s.validateTransition(ctx, enrollmentID, "WITHDRAWN"); err != nil { return err }
	return s.store.UpdateStatus(ctx, enrollmentID, "WITHDRAWN", "")
}

func (s *EnrollmentService) validateTransition(ctx context.Context, id int64, target string) error {
	status, err := s.store.Status(ctx, id)
	if err != nil { return err }
	if status == "COMPLETED" || status == "WITHDRAWN" { return domain.ErrInvalidStatus }
	switch target {
	case "DEFERRED":
		if status != "ENROLLED" && status != "ACTIVE" { return domain.ErrInvalidStatus }
	case "WITHDRAWN":
		if status != "ENROLLED" && status != "ACTIVE" && status != "DEFERRED" { return domain.ErrInvalidStatus }
	case "COMPLETED":
		if status != "ENROLLED" && status != "ACTIVE" { return domain.ErrInvalidStatus }
	default:
		return domain.ErrInvalidStatus
	}
	return nil
}