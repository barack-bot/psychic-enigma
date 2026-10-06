package events

import "context"

type Handler func(context.Context, Event) error

type Subscriber interface {
	Subscribe(context.Context, string, string, Handler) error
}
