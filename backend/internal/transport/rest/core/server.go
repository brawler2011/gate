package core

import (
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/nats-io/nats.go/jetstream"
)

var _ corev1.Handler = (*CoreServer)(nil)

type CoreServer struct {
	identity      IdentityModule
	organizations OrganizationsModule
	contests      ContestsModule
	problems      ProblemsModule
	submissions   SubmissionsModule
	notifications NotificationsModule
	blogs         BlogsModule
	natsJS        jetstream.JetStream
	secureCookies bool
}

func NewCoreServer(
	identity IdentityModule,
	organizations OrganizationsModule,
	contests ContestsModule,
	problems ProblemsModule,
	submissions SubmissionsModule,
	notifications NotificationsModule,
	blogs BlogsModule,
	natsJS jetstream.JetStream,
	secureCookies bool,
) *CoreServer {
	return &CoreServer{
		identity:      identity,
		organizations: organizations,
		contests:      contests,
		problems:      problems,
		submissions:   submissions,
		notifications: notifications,
		blogs:         blogs,
		natsJS:        natsJS,
		secureCookies: secureCookies,
	}
}
