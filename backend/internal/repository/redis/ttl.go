package cache

import "time"

const (
	UserTTL       = 15 * time.Minute
	UsersListTTL  = 5 * time.Minute
	ContestTTL    = 10 * time.Minute
	ProblemTTL    = 30 * time.Minute
	PermissionTTL = 5 * time.Minute
)
