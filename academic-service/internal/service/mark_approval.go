package service

import (
	"context"
	"errors"

	"school-platform/academic-service/internal/domain"
)

type MarkApprovalStore interface {
	Transition(context.Context, int64, string, string, int64, string) error
}

type MarkApprovalService struct{ store MarkApprovalStore }

func NewMarkApprovalService(store MarkApprovalStore) *MarkApprovalService {
	return &MarkApprovalService{store: store}
}

// Approve transitions a SUBMITTED mark to APPROVED.
func (s *MarkApprovalService) Approve(ctx context.Context, markID, hodID int64) error {
	if markID <= 0 || hodID <= 0 {
		return errors.New("mark_id and hod_id are required")
	}
	return s.store.Transition(ctx, markID, "SUBMITTED", "APPROVED", hodID, "")
}

// Reject transitions a SUBMITTED mark to REJECTED, requiring a reason.
func (s *MarkApprovalService) Reject(ctx context.Context, markID, hodID int64, reason string) error {
	if markID <= 0 || hodID <= 0 {
		return errors.New("mark_id and hod_id are required")
	}
	if reason == "" {
		return errors.New("rejection_reason is required")
	}
	return s.store.Transition(ctx, markID, "SUBMITTED", "REJECTED", hodID, reason)
}

var _ = domain.ErrInvalidStatus