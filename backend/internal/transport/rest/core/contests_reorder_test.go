package core_test

import (
	"context"
	"testing"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/stretchr/testify/require"
)

func TestReorderContestProblemsRejectsMissingBody(t *testing.T) {
	server := core.NewCoreServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	err := server.ReorderContestProblems(context.Background(), nil, corev1.ReorderContestProblemsParams{OrgLogin: "org", ContestLogin: "round"})
	require.Error(t, err)
}
