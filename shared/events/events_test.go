package events

import (
	"context"
	"testing"
	"time"
)

func TestStudentEventRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	event, err := NewStudentEvent("event-1", StudentCreated, 2, Student{
		ID:          1,
		AdmissionNo: "ADM-001",
		Name:        "Ada Lovelace",
		Email:       "ada@example.test",
		Phone:       "+254700000000",
		Status:      "active",
	}, createdAt)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := event.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Unmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	student, err := decoded.DecodeStudent()
	if err != nil {
		t.Fatal(err)
	}

	if decoded.ID != "event-1" || decoded.Name != StudentCreated || decoded.Version != 2 {
		t.Fatalf("unexpected event metadata: %+v", decoded)
	}
	if student.ID != 1 || student.Name != "Ada Lovelace" {
		t.Fatalf("unexpected student payload: %+v", student)
	}
}

func TestConnectRetryDoesNotBlockStartup(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _, _ = ConnectRetry(ctx, "nats://127.0.0.1:1")
	}()

	select {
	case <-done:
		// fine: the retry loop stopped within the timeout window.
	case <-time.After(3 * time.Second):
		t.Fatal("ConnectRetry blocked startup by retrying NATS for too long")
	}
}
