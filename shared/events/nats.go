package events

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct{ js nats.JetStreamContext }

func NewNATSPublisher(js nats.JetStreamContext) *NATSPublisher { return &NATSPublisher{js: js} }

func (p *NATSPublisher) Publish(ctx context.Context, event Event) error {
	data, err := event.Marshal()
	if err != nil {
		return err
	}
	ack, err := p.js.PublishMsg(&nats.Msg{Subject: StudentSubject + "." + event.Name, Data: data}, nats.Context(ctx))
	if err != nil {
		return fmt.Errorf("publish student event: %w", err)
	}
	if ack == nil {
		return fmt.Errorf("publish student event: empty acknowledgement")
	}
	return nil
}

type NATSSubscriber struct{ js nats.JetStreamContext }

func NewNATSSubscriber(js nats.JetStreamContext) *NATSSubscriber { return &NATSSubscriber{js: js} }

func (s *NATSSubscriber) Subscribe(ctx context.Context, durable, subject string, handler Handler) error {
	_, err := s.js.Subscribe(subject, func(message *nats.Msg) {
		event, decodeErr := Unmarshal(message.Data)
		if decodeErr != nil {
			log.Printf("student event decode failed: %v", decodeErr)
			terminateOrRetry(message)
			return
		}
		if handleErr := handler(ctx, event); handleErr != nil {
			log.Printf("student event handling failed: %v", handleErr)
			terminateOrRetry(message)
			return
		}
		_ = message.Ack()
	}, nats.Durable(durable), nats.ManualAck(), nats.MaxDeliver(5), nats.BindStream(StudentStream))
	return err
}

func terminateOrRetry(message *nats.Msg) {
	metadata, err := message.Metadata()
	if err == nil && metadata.NumDelivered >= 5 {
		_ = message.Term()
		return
	}
	_ = message.Nak()
}
