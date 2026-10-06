package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	sharedevents "school-platform/shared/events"
)

type Publisher interface {
	Publish(context.Context, sharedevents.Event) error
}

type OutboxPublisher struct {
	db        *sql.DB
	publisher Publisher
	interval  time.Duration
}

func NewOutboxPublisher(db *sql.DB, publisher Publisher, interval time.Duration) *OutboxPublisher {
	if interval <= 0 {
		interval = time.Second
	}
	return &OutboxPublisher{db: db, publisher: publisher, interval: interval}
}

func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishPending(ctx); err != nil {
				log.Printf("student outbox: %v", err)
			}
		}
	}
}

func (p *OutboxPublisher) publishPending(ctx context.Context) error {
	rows, err := p.db.QueryContext(ctx, `
		SELECT event_id, event_type, student_id, version, payload, created_at
		FROM student_events_outbox WHERE published_at IS NULL
		ORDER BY created_at LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var eventID, eventType string
		var studentID int64
		var version int64
		var payload []byte
		var occurredAt time.Time
		if err := rows.Scan(&eventID, &eventType, &studentID, &version, &payload, &occurredAt); err != nil {
			return err
		}
		var student sharedevents.Student
		if err := json.Unmarshal(payload, &student); err != nil {
			return fmt.Errorf("decode outbox %s: %w", eventID, err)
		}
		event, err := sharedevents.NewStudentEvent(eventID, eventType, version, student, occurredAt)
		if err != nil {
			return err
		}
		if err := p.publisher.Publish(ctx, event); err != nil {
			return err
		}
		if _, err := p.db.ExecContext(ctx, `UPDATE student_events_outbox SET published_at = CURRENT_TIMESTAMP WHERE event_id = $1 AND published_at IS NULL`, eventID); err != nil {
			return err
		}
	}
	return rows.Err()
}
