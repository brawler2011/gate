package core

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
)

type testContestsModule struct {
	ContestsUC
	PermissionsUC
	ContestsReader
	ContestDetails
	DraftsUC
	DraftsReader
	AnnouncementsUC
	AnnouncementsReader
	ClarificationsUC
	ClarificationsReader
}

func (m testContestsModule) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	if m.ContestsUC != nil {
		return m.ContestsUC.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	}
	if m.ContestsReader != nil {
		return m.ContestsReader.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	}
	return contestsmodel.Contest{}, nil
}
