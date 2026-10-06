package events

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	StudentCreated = "StudentCreated"
	StudentUpdated = "StudentUpdated"
	StudentDeleted = "StudentDeleted"
	StudentSubject = "school.student"
	StudentStream  = "SCHOOL_STUDENTS"
)

type Event struct {
	ID            string          `json:"event_id"`
	Name          string          `json:"event_type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	Version       int64           `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

type Student struct {
	ID          int64  `json:"student_id"`
	AdmissionNo string `json:"admission_number"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone_number"`
	Status      string `json:"status"`
	Deleted     bool   `json:"deleted,omitempty"`
}

func NewStudentEvent(id, name string, version int64, student Student, occurredAt time.Time) (Event, error) {
	payload, err := json.Marshal(student)
	if err != nil {
		return Event{}, fmt.Errorf("marshal student event: %w", err)
	}
	return Event{
		ID:            id,
		Name:          name,
		AggregateType: "student",
		AggregateID:   fmt.Sprintf("%d", student.ID),
		Version:       version,
		OccurredAt:    occurredAt,
		Payload:       json.RawMessage(payload),
	}, nil
}

func (e Event) DecodeStudent() (Student, error) {
	var student Student
	if err := json.Unmarshal(e.Payload, &student); err != nil {
		return Student{}, fmt.Errorf("decode student event: %w", err)
	}
	return student, nil
}

func (e Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

func Unmarshal(data []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, fmt.Errorf("decode event: %w", err)
	}
	return event, nil
}
