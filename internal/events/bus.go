// Package events menyediakan Event Bus untuk arsitektur event-driven
// (EPIC V2.2). Publisher (service) dan consumer (worker) terdekupling penuh:
// mereka hanya sepakat pada nama topik dan bentuk payload.
package events

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Nama topik yang dikenal sistem.
const (
	TopicTransactionCreated = "transaction.created"
)

// TransactionCreatedEvent payload event setelah transaksi baru berhasil commit.
type TransactionCreatedEvent struct {
	UserID        uuid.UUID `json:"user_id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	Type          string    `json:"type"` // INCOME / EXPENSE
	Amount        int64     `json:"amount"`
	TrxDate       time.Time `json:"trx_date"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// Handler dipanggil worker saat event diterima. Payload adalah JSON mentah.
type Handler func(ctx context.Context, payload []byte)

// EventBus abstraksi message broker. Implementasi: InMemoryBus (default,
// tanpa infrastruktur eksternal) dan RedisBus (Redis Pub/Sub).
type EventBus interface {
	// Publish mengirim event secara asynchronous (non-blocking best effort).
	Publish(ctx context.Context, topic string, payload any) error
	// Subscribe mendaftarkan handler untuk sebuah topik.
	Subscribe(topic string, handler Handler) error
	// Start menjalankan listener (no-op untuk in-memory).
	Start(ctx context.Context) error
	Close() error
}

// ---------------------------------------------------------------------------
// InMemoryBus — pub/sub antar-goroutine pada satu proses. Cocok untuk dev,
// test, dan deployment single-instance.
//
// Event diproses SEKUENSIAL oleh satu dispatcher (urutan kausal terjaga dan
// consumer yang punya guard idempotensi tidak berpacu dengan dirinya sendiri),
// meniru semantik Redis Pub/Sub yang juga dikonsumsi berurutan.
// ---------------------------------------------------------------------------

type queuedEvent struct {
	topic    string
	data     []byte
	handlers []Handler
}

type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	queue    chan queuedEvent
	once     sync.Once
}

func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[string][]Handler),
		queue:    make(chan queuedEvent, 1024),
	}
}

func (b *InMemoryBus) startDispatcher() {
	b.once.Do(func() {
		go func() {
			for ev := range b.queue {
				for _, h := range ev.handlers {
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("[bus] panic handler topik %q: %v", ev.topic, r)
							}
						}()
						h(context.Background(), ev.data)
					}()
				}
			}
		}()
	})
}

func (b *InMemoryBus) Publish(ctx context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	b.mu.RLock()
	handlers := make([]Handler, len(b.handlers[topic]))
	copy(handlers, b.handlers[topic])
	b.mu.RUnlock()
	if len(handlers) == 0 {
		return nil
	}

	b.startDispatcher()
	select {
	case b.queue <- queuedEvent{topic: topic, data: data, handlers: handlers}:
	default:
		// Best-effort: antrean penuh → event di-drop (advisory, bukan ledger).
		log.Printf("[bus] antrean penuh, event topik %q di-drop", topic)
	}
	return nil
}

func (b *InMemoryBus) Subscribe(topic string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

func (b *InMemoryBus) Start(_ context.Context) error { return nil }
func (b *InMemoryBus) Close() error                  { return nil }

// ---------------------------------------------------------------------------
// RedisBus — Redis Pub/Sub untuk deployment multi-instance.
// ---------------------------------------------------------------------------

const redisChannelPrefix = "caupocket:"

type RedisBus struct {
	client   *redis.Client
	mu       sync.RWMutex
	handlers map[string][]Handler
	subs     []*redis.PubSub
}

func NewRedisBus(addr, password string, db int) *RedisBus {
	return &RedisBus{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
		handlers: make(map[string][]Handler),
	}
}

func (b *RedisBus) Publish(ctx context.Context, topic string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, redisChannelPrefix+topic, data).Err()
}

func (b *RedisBus) Subscribe(topic string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

func (b *RedisBus) Start(ctx context.Context) error {
	if err := b.client.Ping(ctx).Err(); err != nil {
		return err
	}
	b.mu.RLock()
	topics := make([]string, 0, len(b.handlers))
	for topic := range b.handlers {
		topics = append(topics, redisChannelPrefix+topic)
	}
	b.mu.RUnlock()
	if len(topics) == 0 {
		return nil
	}

	sub := b.client.Subscribe(ctx, topics...)
	if _, err := sub.Receive(ctx); err != nil {
		return err
	}
	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()

	go func() {
		for msg := range sub.Channel() {
			topic := msg.Channel[len(redisChannelPrefix):]
			b.mu.RLock()
			handlers := make([]Handler, len(b.handlers[topic]))
			copy(handlers, b.handlers[topic])
			b.mu.RUnlock()

			data := []byte(msg.Payload)
			for _, h := range handlers {
				h := h
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[bus:redis] panic handler topik %q: %v", topic, r)
						}
					}()
					h(context.Background(), data)
				}()
			}
		}
	}()
	return nil
}

func (b *RedisBus) Close() error {
	for _, sub := range b.subs {
		_ = sub.Close()
	}
	return b.client.Close()
}
