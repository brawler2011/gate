package observer

import (
	"testing"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/stretchr/testify/require"
)

func TestMalformedObserverEventIsPermanent(t *testing.T) {
	_, err := parseEvent[submissionsmodel.SubmissionCreatedEvent]([]byte("invalid"))
	require.Error(t, err)
	require.True(t, eventdata.IsPermanent(err))
}
