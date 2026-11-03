package contests

type contestEventsTestStore struct {
	AnnouncementsRepo
	ClarificationsRepo
	OutboxRepo
}
