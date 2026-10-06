package service

import (
	"context"
	"errors"
	"testing"

	"school-platform/academic-service/internal/domain"
)

type enrollmentStoreStub struct {
	status string
	updated string
}

func (s *enrollmentStoreStub) ListByStudent(context.Context, int64) ([]domain.Enrollment, error) { return nil, nil }
func (s *enrollmentStoreStub) Upsert(context.Context, domain.Enrollment) (int64, error) { return 1, nil }
func (s *enrollmentStoreStub) UpdateStatus(_ context.Context, _ int64, status, _ string) error { s.updated = status; return nil }
func (s *enrollmentStoreStub) Status(context.Context, int64) (string, error) { return s.status, nil }

func TestEnrollmentTransitions(t *testing.T) {
	tests := []struct {
		name   string
		status string
		action func(*EnrollmentService) error
		want   error
	}{
		{"active can defer", "ACTIVE", func(s *EnrollmentService) error { return s.Defer(context.Background(), 1, "medical") }, nil},
		{"deferred can withdraw", "DEFERRED", func(s *EnrollmentService) error { return s.Withdraw(context.Background(), 1) }, nil},
		{"completed is terminal", "COMPLETED", func(s *EnrollmentService) error { return s.Withdraw(context.Background(), 1) }, domain.ErrInvalidStatus},
		{"withdrawn is terminal", "WITHDRAWN", func(s *EnrollmentService) error { return s.Defer(context.Background(), 1, "reason") }, domain.ErrInvalidStatus},
		{"defer needs reason", "ACTIVE", func(s *EnrollmentService) error { return s.Defer(context.Background(), 1, "") }, errors.New("deferred reason is required")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &enrollmentStoreStub{status: test.status}
			service := NewEnrollmentService(store)
			err := test.action(service)
			if test.want != nil && (err == nil || err.Error() != test.want.Error()) { t.Fatalf("want %v, got %v", test.want, err) }
			if test.want == nil && err != nil { t.Fatalf("unexpected error: %v", err) }
		})
	}
}
