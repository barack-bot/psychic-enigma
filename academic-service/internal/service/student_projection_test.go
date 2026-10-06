package service

import (
	"context"
	"testing"
	"time"

	"school-platform/shared/events"
)

type memoryAcademicStore struct {
	students map[int64]AcademicStudent
	seen     map[string]bool
}

func newMemoryAcademicStore() *memoryAcademicStore {
	return &memoryAcademicStore{students: map[int64]AcademicStudent{}, seen: map[string]bool{}}
}
func (s *memoryAcademicStore) ApplyStudentEvent(_ context.Context, event events.Event, student events.Student) error {
	if s.seen[event.ID] {
		return nil
	}
	s.seen[event.ID] = true
	current := s.students[student.ID]
	if event.Version > current.SourceVersion {
		s.students[student.ID] = AcademicStudent{student.ID, student.AdmissionNo, student.Name, student.Status, event.Version}
	}
	return nil
}
func (s *memoryAcademicStore) ListStudents(_ context.Context) ([]AcademicStudent, error) {
	out := make([]AcademicStudent, 0, len(s.students))
	for _, student := range s.students {
		out = append(out, student)
	}
	return out, nil
}
func academicEvent(t *testing.T, id string, version int64, name string, student events.Student) events.Event {
	t.Helper()
	event, err := events.NewStudentEvent(id, name, version, student, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestStudentProjectionIsIdempotentAndVersioned(t *testing.T) {
	store := newMemoryAcademicStore()
	projection := NewStudentProjection(store)
	ctx := context.Background()
	newer := academicEvent(t, "event-2", 2, events.StudentUpdated, events.Student{ID: 1, AdmissionNo: "A-1", Name: "New Name", Status: "ACTIVE"})
	older := academicEvent(t, "event-1", 1, events.StudentCreated, events.Student{ID: 1, AdmissionNo: "A-1", Name: "Old Name", Status: "ACTIVE"})
	if err := projection.Consume(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if err := projection.Consume(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := projection.Consume(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if got := store.students[1]; got.Name != "New Name" || got.SourceVersion != 2 {
		t.Fatalf("unexpected projection: %+v", got)
	}
}

