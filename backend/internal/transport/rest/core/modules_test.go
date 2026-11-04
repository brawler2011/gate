package core

type testContestsModule struct {
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

type testProblemsModule struct {
	ProblemsUC
	ProblemsReader
	WorkshopUC
	ImportUC
	PublishUC
	PackagesReader
}
