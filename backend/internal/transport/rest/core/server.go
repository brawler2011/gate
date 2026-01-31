package core

import (
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/usecase"
)

type CoreServer struct {
	contestsUC      interfaces.ContestsUC
	permissionsUC   interfaces.PermissionsUC
	submissionsUC   interfaces.SubmissionsUC
	usersUC         interfaces.UsersUC
	problemsUC      interfaces.ProblemsUC
	organizationsUC interfaces.OrganizationsUC
	teamsUC         interfaces.TeamsUC
	blogsUC         *usecase.BlogsUseCase
	avatarsUC       *usecase.AvatarsUseCase
	importUC        *usecase.ProblemImportUseCase
	publishUC       *usecase.ProblemPublishUseCase
}

func NewCoreServer(
	contestsUC interfaces.ContestsUC,
	permissionsUC interfaces.PermissionsUC,
	submissionsUC interfaces.SubmissionsUC,
	usersUC interfaces.UsersUC,
	problemsUC interfaces.ProblemsUC,
	organizationsUC interfaces.OrganizationsUC,
	teamsUC interfaces.TeamsUC,
	blogsUC *usecase.BlogsUseCase,
	avatarsUC *usecase.AvatarsUseCase,
	importUC *usecase.ProblemImportUseCase,
	publishUC *usecase.ProblemPublishUseCase,
) *CoreServer {
	return &CoreServer{
		contestsUC:      contestsUC,
		permissionsUC:   permissionsUC,
		submissionsUC:   submissionsUC,
		usersUC:         usersUC,
		problemsUC:      problemsUC,
		organizationsUC: organizationsUC,
		teamsUC:         teamsUC,
		blogsUC:         blogsUC,
		avatarsUC:       avatarsUC,
		importUC:        importUC,
		publishUC:       publishUC,
	}
}
