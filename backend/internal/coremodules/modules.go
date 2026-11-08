package coremodules

import (
	"context"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
)

type Identity struct {
	core.AuthUC
	core.UsersUC
	core.UsersReader
	core.AvatarsUC
}

type Organizations struct {
	core.OrganizationsUC
	core.TeamsUC
}

type Contests struct {
	core.ContestsUC
	core.PermissionsUC
	core.ContestsReader
	core.ContestDetails
	core.DraftsUC
	core.DraftsReader
	core.AnnouncementsUC
	core.AnnouncementsReader
	core.ClarificationsUC
	core.ClarificationsReader
}

func (c Contests) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	if c.ContestsUC != nil {
		return c.ContestsUC.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	}
	if c.ContestsReader != nil {
		return c.ContestsReader.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	}
	return contestsmodel.Contest{}, nil
}

type Problems struct {
	core.ProblemsUC
	core.ProblemsReader
	core.WorkshopUC
	core.ImportUC
	core.PublishUC
	core.PackagesReader
}

type Submissions struct {
	core.SubmissionsUC
}

type Notifications struct {
	core.NotificationsUC
	core.NotificationsReader
}

type Blogs struct {
	core.BlogsUC
	core.BlogsReader
}

var (
	_ core.IdentityModule      = Identity{}
	_ core.OrganizationsModule = Organizations{}
	_ core.ContestsModule      = Contests{}
	_ core.ProblemsModule      = Problems{}
	_ core.SubmissionsModule   = Submissions{}
	_ core.NotificationsModule = Notifications{}
	_ core.BlogsModule         = Blogs{}
)
