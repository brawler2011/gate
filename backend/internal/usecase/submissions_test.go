package usecase

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func TestRejudgeSubmissions_NilContestID(t *testing.T) {
	uc := &SubmissionsUseCase{}
	_, err := uc.RejudgeSubmissions(context.Background(), models.RejudgeFilter{
		ContestID: uuid.Nil,
	})
	if err == nil {
		t.Fatal("expected error for nil contest ID, got nil")
	}
}
