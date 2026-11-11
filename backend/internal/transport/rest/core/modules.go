package core

type IdentityModule interface {
	AuthUC
	UsersUC
	UsersReader
	AvatarsUC
}

type OrganizationsModule interface {
	OrganizationsUC
	TeamsUC
}

type ContestsModule interface {
	ContestsUC
	PermissionsUC
	ContestsReader
	ContestDetails
	DraftsUC
	DraftsReader
	AnnouncementsUC
	AnnouncementsReader
	ClarificationsUC
	ClarificationsReader
}

type ProblemsModule interface {
	ProblemsUC
	ProblemsReader
	ProblemDraftUC
	PackagesReader
}

type SubmissionsModule interface {
	SubmissionsUC
}

type NotificationsModule interface {
	NotificationsUC
	NotificationsReader
}

type BlogsModule interface {
	BlogsUC
	BlogsReader
}
