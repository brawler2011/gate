package core

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg/booklet"
	"github.com/nats-io/nats.go/jetstream"
)

var _ corev1.Handler = (*CoreServer)(nil)

type CoreServer struct {
	packagesReader       PackagesReader
	usersReader          UsersReader
	problemsReader       ProblemsReader
	contestsReader       ContestsReader
	blogsReader          BlogsReader
	draftsReader         DraftsReader
	notificationsReader  NotificationsReader
	clarificationsReader ClarificationsReader
	authUC               interfaces.AuthUC
	contestsUC           interfaces.ContestsUC
	permissionsUC        interfaces.PermissionsUC
	submissionsUC        interfaces.SubmissionsUC
	usersUC              interfaces.UsersUC
	problemsUC           interfaces.ProblemsUC
	organizationsUC      interfaces.OrganizationsUC
	teamsUC              interfaces.TeamsUC
	workshopUC           interfaces.WorkshopUC
	blogsUC              *usecase.BlogsUseCase
	avatarsUC            *usecase.AvatarsUseCase
	importUC             *usecase.ProblemImportUseCase
	publishUC            *usecase.ProblemPublishUseCase
	draftsUC             interfaces.DraftsUC
	notificationsUC      interfaces.NotificationsUC
	announcementsUC      *usecase.AnnouncementsUseCase
	announcementsReader  AnnouncementsReader
	clarificationsUC     *usecase.ClarificationsUseCase
	natsJS               jetstream.JetStream
	bookletCompiler      booklet.Compiler
}

func NewCoreServer(
	authUC interfaces.AuthUC,
	contestsUC interfaces.ContestsUC,
	permissionsUC interfaces.PermissionsUC,
	submissionsUC interfaces.SubmissionsUC,
	usersUC interfaces.UsersUC,
	problemsUC interfaces.ProblemsUC,
	organizationsUC interfaces.OrganizationsUC,
	teamsUC interfaces.TeamsUC,
	workshopUC interfaces.WorkshopUC,
	blogsUC *usecase.BlogsUseCase,
	avatarsUC *usecase.AvatarsUseCase,
	importUC *usecase.ProblemImportUseCase,
	publishUC *usecase.ProblemPublishUseCase,
	draftsUC interfaces.DraftsUC,
	notificationsUC interfaces.NotificationsUC,
	announcementsUC *usecase.AnnouncementsUseCase,
	announcementsReader AnnouncementsReader,
	clarificationsUC *usecase.ClarificationsUseCase,
	natsJS jetstream.JetStream,
	bookletCompiler booklet.Compiler,
	usersReader UsersReader,
	problemsReader ProblemsReader,
	contestsReader ContestsReader,
	blogsReader BlogsReader,
	draftsReader DraftsReader,
	notificationsReader NotificationsReader,
	clarificationsReader ClarificationsReader,
	packagesReader PackagesReader,
) *CoreServer {
	return &CoreServer{
		packagesReader:       packagesReader,
		usersReader:          usersReader,
		problemsReader:       problemsReader,
		contestsReader:       contestsReader,
		blogsReader:          blogsReader,
		draftsReader:         draftsReader,
		notificationsReader:  notificationsReader,
		clarificationsReader: clarificationsReader,
		authUC:               authUC,
		contestsUC:           contestsUC,
		permissionsUC:        permissionsUC,
		submissionsUC:        submissionsUC,
		usersUC:              usersUC,
		problemsUC:           problemsUC,
		organizationsUC:      organizationsUC,
		teamsUC:              teamsUC,
		workshopUC:           workshopUC,
		blogsUC:              blogsUC,
		avatarsUC:            avatarsUC,
		importUC:             importUC,
		publishUC:            publishUC,
		draftsUC:             draftsUC,
		notificationsUC:      notificationsUC,
		announcementsUC:      announcementsUC,
		announcementsReader:  announcementsReader,
		clarificationsUC:     clarificationsUC,
		natsJS:               natsJS,
		bookletCompiler:      bookletCompiler,
	}
}
