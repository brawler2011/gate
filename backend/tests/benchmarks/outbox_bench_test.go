//go:build integration
// +build integration

package benchmarks

import (
	"context"
	"testing"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	"github.com/google/uuid"
)

func BenchmarkOutbox_CreateEvent(b *testing.B) {
	ctx := context.Background()
	payload := []byte(`{"event": "submission_created", "status": "pending"}`)
	headers := map[string]string{"source": "bench", "version": "1.0"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := outboxRepo.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: testContestID,
			EventType:   "benchmark.event.created",
			Payload:     payload,
			Headers:     headers,
		})
		if err != nil {
			b.Fatalf("failed to create outbox event: %v", err)
		}
	}
}

func BenchmarkOutbox_PickEvents(b *testing.B) {
	ctx := context.Background()
	payload := []byte(`{"event": "pick_test"}`)
	headers := map[string]string{"source": "bench"}

	// Ensure sufficient pending events exist
	for i := 0; i < 200; i++ {
		_ = outboxRepo.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{
			Id:          uuid.New(),
			AggregateID: uuid.New(),
			EventType:   "benchmark.pick.event",
			Payload:     payload,
			Headers:     headers,
		})
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		events, err := outboxRepo.PickEvents(ctx, 10, 30)
		if err != nil {
			b.Fatalf("failed to pick outbox events: %v", err)
		}
		if len(events) == 0 {
			b.StopTimer()
			for j := 0; j < 50; j++ {
				_ = outboxRepo.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{
					Id:          uuid.New(),
					AggregateID: uuid.New(),
					EventType:   "benchmark.pick.event",
					Payload:     payload,
					Headers:     headers,
				})
			}
			b.StartTimer()
		}
	}
}

func BenchmarkOutbox_PickAndMarkCompleted(b *testing.B) {
	ctx := context.Background()
	payload := []byte(`{"event": "lifecycle_test"}`)
	headers := map[string]string{"source": "bench"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Pre-insert batch of events with unique aggregate IDs so they are pickable
		for j := 0; j < 10; j++ {
			_ = outboxRepo.CreateEvent(ctx, &eventdata.CreateOutboxEventParams{
				Id:          uuid.New(),
				AggregateID: uuid.New(),
				EventType:   "benchmark.lifecycle.event",
				Payload:     payload,
				Headers:     headers,
			})
		}
		b.StartTimer()

		events, err := outboxRepo.PickEvents(ctx, 10, 30)
		if err != nil {
			b.Fatalf("failed to pick events: %v", err)
		}

		for _, e := range events {
			if err := outboxRepo.MarkAsCompleted(ctx, e.Id, e.LeaseToken); err != nil {
				b.Fatalf("failed to mark outbox event completed: %v", err)
			}
		}
	}
}
