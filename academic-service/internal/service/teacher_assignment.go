package service

import (
	"context"
	"errors"

	"school-platform/academic-service/internal/domain"
)

type TeacherAssignmentStore interface {
	ListClassSubjects(context.Context, int64) ([]domain.ClassSubject, error)
	AssignTeacher(context.Context, int64, int64) error
}

type TeacherAssignmentService struct{ store TeacherAssignmentStore }

func NewTeacherAssignmentService(store TeacherAssignmentStore) *TeacherAssignmentService {
	return &TeacherAssignmentService{store: store}
}

func (s *TeacherAssignmentService) Assign(ctx context.Context, classSubjectID, teacherID int64) error {
	if classSubjectID <= 0 || teacherID <= 0 {
		return errors.New("class_subject_id and teacher_id are required")
	}
	return s.store.AssignTeacher(ctx, classSubjectID, teacherID)
}

func (s *TeacherAssignmentService) ListForClass(ctx context.Context, classOfferingID int64) ([]domain.ClassSubject, error) {
	return s.store.ListClassSubjects(ctx, classOfferingID)
}