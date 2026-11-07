package access

import (
	"context"
	"testing"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUseProblemRequiresAuthenticatedReadAccess(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		public, guest, admin, owner bool
		role                        problemsmodel.ProblemRole
		want                        bool
	}{
		{name: "public", public: true, want: true},
		{name: "private outsider"},
		{name: "viewer", role: problemsmodel.ProblemRoleViewer, want: true},
		{name: "moderator", role: problemsmodel.ProblemRoleModerator, want: true},
		{name: "member owner", role: problemsmodel.ProblemRoleOwner, want: true},
		{name: "owner", owner: true, want: true},
		{name: "global admin", admin: true, want: true},
		{name: "guest public", public: true, guest: true},
		{name: "guest private", guest: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupPermissionsFixture()
			actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			if tc.admin {
				actor.Role = accessmodel.UserRoleAdmin
			}
			if tc.guest {
				actor = accessmodel.Guest
			} else {
				f.usersUC.users[actor.Id] = actor
			}
			problem := problemsmodel.Problem{ID: f.problemID, OrganizationID: f.orgID, Visibility: problemsmodel.ProblemVisibilityPrivate}
			if tc.public {
				problem.Visibility = problemsmodel.ProblemVisibilityPublic
			}
			if tc.owner {
				problem.OwnerID = &actor.Id
			}
			f.problems.problems[problem.ID] = problem
			if tc.role != "" {
				f.problems.members[problem.ID.String()+":"+actor.Id.String()] = problemsmodel.ProblemMember{Role: tc.role}
			}
			allowed, err := f.permUC.HasProblemPermission(context.Background(), problem.ID, actor.Id, accessmodel.ActionUseProblem)
			require.NoError(t, err)
			require.Equal(t, tc.want, allowed)
		})
	}
}
