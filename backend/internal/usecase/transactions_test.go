package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brawler2011/gate/backend/internal/judging"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
)

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }

type identityTestStore struct {
	interfaces.UsersRepo
	interfaces.AuthRepo
	interfaces.ContestsRepo
}
type organizationTestStore struct {
	interfaces.OrganizationsRepo
	interfaces.UsersRepo
}
type contestEventsTestStore struct {
	interfaces.AnnouncementsRepo
	interfaces.ClarificationsRepo
	interfaces.OutboxRepo
}
type submissionTestStore struct {
	interfaces.SubmissionsRepo
	interfaces.OutboxRepo
}

func recordTestEvent(ctx context.Context, e events.Event, create func(context.Context, *models.CreateOutboxEventParams) error) error {
	data, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("encode test event: %w", err)
	}
	return create(ctx, &models.CreateOutboxEventParams{Id: uuid.New(), AggregateID: e.AggregateID, EventType: e.Type, Payload: data, Headers: map[string]string{requestid.Header: requestid.Get(ctx)}})
}

func (submissionTestStore) ScheduleJob(_ context.Context, id uuid.UUID) (judging.Job, error) {
	return judging.Job{ID: uuid.New(), SubmissionID: id, Generation: 1}, nil
}
