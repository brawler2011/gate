package core

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	blogs "github.com/brawler2011/gate/backend/internal/blogs"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	problems "github.com/brawler2011/gate/backend/internal/problems"
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
	authUC               AuthUC
	contestsUC           ContestsUC
	permissionsUC        PermissionsUC
	submissionsUC        SubmissionsUC
	usersUC              UsersUC
	problemsUC           ProblemsUC
	organizationsUC      OrganizationsUC
	teamsUC              TeamsUC
	workshopUC           WorkshopUC
	blogsUC              *blogs.BlogsUseCase
	avatarsUC            *identity.AvatarsUseCase
	importUC             *problems.ProblemImportUseCase
	publishUC            *problems.ProblemPublishUseCase
	draftsUC             DraftsUC
	notificationsUC      NotificationsUC
	announcementsUC      *contests.AnnouncementsUseCase
	announcementsReader  AnnouncementsReader
	clarificationsUC     *contests.ClarificationsUseCase
	natsJS               jetstream.JetStream
	bookletCompiler      booklet.Compiler
}

func NewCoreServer(
	authUC AuthUC,
	contestsUC ContestsUC,
	permissionsUC PermissionsUC,
	submissionsUC SubmissionsUC,
	usersUC UsersUC,
	problemsUC ProblemsUC,
	organizationsUC OrganizationsUC,
	teamsUC TeamsUC,
	workshopUC WorkshopUC,
	blogsUC *blogs.BlogsUseCase,
	avatarsUC *identity.AvatarsUseCase,
	importUC *problems.ProblemImportUseCase,
	publishUC *problems.ProblemPublishUseCase,
	draftsUC DraftsUC,
	notificationsUC NotificationsUC,
	announcementsUC *contests.AnnouncementsUseCase,
	announcementsReader AnnouncementsReader,
	clarificationsUC *contests.ClarificationsUseCase,
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
