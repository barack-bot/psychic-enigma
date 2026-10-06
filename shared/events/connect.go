package events

import (
	"context"
	"time"
	"github.com/nats-io/nats.go"
)

func ConnectRetry(ctx context.Context, url string) (*nats.Conn, nats.JetStreamContext, error) {
	var last error
	for attempt := 0; attempt < 4; attempt++ {
		nc, js, err := Connect(url)
		if err == nil {
			return nc, js, nil
		}
		last = err
		if attempt < 3 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	return nil, nil, last
}

func Connect(url string) (*nats.Conn, nats.JetStreamContext, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     StudentStream,
		Subjects: []string{StudentSubject + ".>"},
		Storage:  nats.FileStorage,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		nc.Close()
		return nil, nil, err
	}
	return nc, js, nil
}
