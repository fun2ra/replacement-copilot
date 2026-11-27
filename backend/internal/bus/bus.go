// Package bus is an in-process stand-in for Kafka / a message broker, so the
// demo can show the event-driven shape (ReplacementReserved is published and
// consumed by other services) without running infrastructure.
package bus

import (
	"sync"
	"time"
)

type Event struct {
	Type    string    `json:"type"`
	At      time.Time `json:"at"`
	Payload any       `json:"payload"`
}

type Bus struct {
	mu   sync.RWMutex
	subs []func(Event)
}

func New() *Bus { return &Bus{} }

func (b *Bus) Subscribe(fn func(Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, fn)
}

func (b *Bus) Publish(typ string, payload any) Event {
	e := Event{Type: typ, At: time.Now(), Payload: payload}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, fn := range b.subs {
		go fn(e) // async, like a real consumer
	}
	return e
}
