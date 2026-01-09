package core

import (
	"github.com/gate149/core/internal/domain/interfaces"
)

type CoreServer struct {
	contestsUC    interfaces.ContestsUC
	permissionsUC interfaces.PermissionsUC
	submissionsUC interfaces.SubmissionsUC
	usersUC       interfaces.UsersUC
	problemsUC    interfaces.ProblemsUC
}

func NewCoreServer(
	contestsUC interfaces.ContestsUC,
	permissionsUC interfaces.PermissionsUC,
	submissionsUC interfaces.SubmissionsUC,
	usersUC interfaces.UsersUC,
	problemsUC interfaces.ProblemsUC,
) *CoreServer {
	return &CoreServer{
		contestsUC:    contestsUC,
		permissionsUC: permissionsUC,
		submissionsUC: submissionsUC,
		usersUC:       usersUC,
		problemsUC:    problemsUC,
	}
}
