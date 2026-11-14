package core

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/nats-io/nats.go/jetstream"
)

var _ corev1.Handler = (*CoreServer)(nil)

// Services lists the application services the REST handlers call. Each field
// is the use case or read store that owns the operation; there is no facade in
// between.
type Services struct {
	Auth      AuthUC
	Users     UsersUC
	UserReads UsersReader
	Avatars   AvatarsUC

	Organizations OrganizationsUC
	Teams         TeamsUC

	Contests           ContestsUC
	ContestPermissions PermissionsUC
	ContestDetails     ContestDetails
	Drafts             DraftsUC
	Announcements      AnnouncementsUC
	Clarifications     ClarificationsUC

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
	languages     map[string]sandbox.Language
}

func NewCoreServer(services Services, secureCookies bool) *CoreServer {
	return &CoreServer{Services: services, secureCookies: secureCookies}
}

// WithLanguages sets the languages reported by GetLanguages.
func (h *CoreServer) WithLanguages(languages map[string]sandbox.Language) *CoreServer {
	h.languages = languages
	return h
}
