package core

import (
	"github.com/gate149/core/internal/domain/interfaces"
)

type CoreServer struct {
	contestsUC       interfaces.ContestsUC
	permissionsUC    interfaces.PermissionsUC
	submissionsUC    interfaces.SubmissionsUC
	usersUC          interfaces.UsersUC
	problemsUC       interfaces.ProblemsUC
	organizationsUC  interfaces.OrganizationsUC
	teamsUC          interfaces.TeamsUC
}

func NewCoreServer(
	contestsUC interfaces.ContestsUC,
	permissionsUC interfaces.PermissionsUC,
	submissionsUC interfaces.SubmissionsUC,
	usersUC interfaces.UsersUC,
	problemsUC interfaces.ProblemsUC,
	organizationsUC interfaces.OrganizationsUC,
	teamsUC interfaces.TeamsUC,
) *CoreServer {
	return &CoreServer{
		contestsUC:      contestsUC,
		permissionsUC:   permissionsUC,
		submissionsUC:   submissionsUC,
		usersUC:         usersUC,
		problemsUC:      problemsUC,
		organizationsUC: organizationsUC,
		teamsUC:         teamsUC,
	}
}
