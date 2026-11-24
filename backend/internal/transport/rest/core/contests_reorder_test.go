package core_test

import (
	"context"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReorderContestProblemsRejectsMissingBody(t *testing.T) {
	server := core.NewCoreServer(core.Services{}, false)
	err := server.ReorderContestProblems(context.Background(), nil, corev1.ReorderContestProblemsParams{ContestID: uuid.New()})
	require.Error(t, err)
}
