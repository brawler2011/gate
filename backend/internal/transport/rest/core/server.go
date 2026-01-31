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
	}
}
