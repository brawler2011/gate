package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/nats-io/nats.go/jetstream"
)

var _ corev1.Handler = (*CoreServer)(nil)

type Services struct {
	Auth      AuthUC
	Users     UsersUC
	UserReads UsersReader
	Avatars   AvatarsUC

	Organizations OrganizationsUC
	Teams         TeamsUC

	Contests       ContestsUC
	Access         access.Loader
	ContestDetails ContestDetails
	Drafts         DraftsUC
	Announcements  AnnouncementsUC
	Clarifications ClarificationsUC

	Problems      ProblemsUC
	ProblemDrafts ProblemDraftUC
	Packages      PackagesReader

	Submissions SubmissionsUC

	Notifications     NotificationsUC
	NotificationReads NotificationsReader

	Blogs     BlogsUC
	BlogReads BlogsReader

	NATS         jetstream.JetStream
	Capabilities CapabilityProjector
}

type CoreServer struct {
	Services
	secureCookies bool
	languages     *sandbox.Config
}

func NewCoreServer(services Services, secureCookies bool) *CoreServer {
	return &CoreServer{Services: services, secureCookies: secureCookies}
}

func (h *CoreServer) WithLanguages(languages *sandbox.Config) *CoreServer {
	h.languages = languages
	return h
}

func (s *CoreServer) NewError(ctx context.Context, err error) *corev1.ProblemStatusCodeWithHeaders {
	return middleware.NewProblem(ctx, err)
}
