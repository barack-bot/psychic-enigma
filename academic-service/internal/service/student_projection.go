package service

import (
	"context"
	"fmt"

	"school-platform/shared/events"
)

type AcademicStudent struct {
	ID            int64
	AdmissionNo   string
	Name          string
	Status        string
	SourceVersion int64
}

type AcademicStudentStore interface {
	ApplyStudentEvent(context.Context, events.Event, events.Student) error
	ListStudents(context.Context) ([]AcademicStudent, error)
}

type StudentProjection struct{ store AcademicStudentStore }

func NewStudentProjection(store AcademicStudentStore) *StudentProjection {
	return &StudentProjection{store: store}
}

func (p *StudentProjection) Consume(ctx context.Context, event events.Event) error {
	if event.Name != events.StudentCreated && event.Name != events.StudentUpdated && event.Name != events.StudentDeleted {
		return fmt.Errorf("unsupported student event %q", event.Name)
	}
	student, err := event.DecodeStudent()
	if err != nil {
		return err
	}
	return p.store.ApplyStudentEvent(ctx, event, student)
}

