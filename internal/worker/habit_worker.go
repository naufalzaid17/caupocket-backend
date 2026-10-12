// Package worker berisi background consumer event (EPIC V2.2).
package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"

	"caupocket-api/internal/events"
	"caupocket-api/internal/service"
)

// HabitWorker me-listen topik "transaction.created" dan menjalankan aturan
// habit engine secara asinkron.
type HabitWorker struct {
	bus   events.EventBus
	habit service.HabitService
}

func NewHabitWorker(bus events.EventBus, habit service.HabitService) *HabitWorker {
	return &HabitWorker{bus: bus, habit: habit}
}

// Start mendaftarkan handler lalu menyalakan listener bus.
func (w *HabitWorker) Start(ctx context.Context) error {
	if err := w.bus.Subscribe(events.TopicTransactionCreated, w.handle); err != nil {
		return err
	}
	return w.bus.Start(ctx)
}

func (w *HabitWorker) handle(ctx context.Context, payload []byte) {
	var event events.TransactionCreatedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("[habit-worker] payload tidak valid: %v", err)
		return
	}
	if event.UserID == uuid.Nil {
		return
	}

	evalCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := w.habit.HandleTransactionCreated(evalCtx, event.UserID); err != nil {
		log.Printf("[habit-worker] gagal memproses %s user %s: %v",
			events.TopicTransactionCreated, event.UserID, err)
	}
}
