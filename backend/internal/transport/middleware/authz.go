package middleware

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	ogenMiddleware "github.com/ogen-go/ogen/middleware"
)

type level int

const (
	public level = iota
	authenticated
	admin
)

// Login and admin gates only; resource permissions are checked by use cases.
var endpointPolicies = map[string]level{
	"GetHealth":          public,
	"ListPosts":          public,
	"GetPostById":        public,
	"GetPostImage":       public,
	"ListProblems":       public,
	"ListUsers":          authenticated,
	"GetUser":            public,
	"GetUserAvatar":      public,
	"GetLanguages":       public,
	"GetSystemLibrary":   public,
	"Register":           public,
	"Login":              public,
	"Logout":             public,
	"VerifyEmail":        public,
	"ResendVerification": public,
	"ForgotPassword":     public,
	"ResetPassword":      public,
	"ConfirmEmailChange": public,

	"GetMe":              authenticated,
	"ChangePassword":     authenticated,
	"RequestEmailChange": authenticated,
	"CreateContest":      authenticated,
	"ListContests":       public,

	"ListOrganizations":              authenticated,
	"CreateOrganization":             authenticated,
	"GetOrganization":                authenticated,
	"UpdateOrganization":             authenticated,
	"DeleteOrganization":             authenticated,
	"ListOrganizationMembers":        authenticated,
	"AddOrganizationMember":          authenticated,
	"RemoveOrganizationMember":       authenticated,
	"BatchCreateOrganizationUsers":   authenticated,
	"ListOrganizationJoinRequests":   authenticated,
	"CreateOrganizationJoinRequest":  authenticated,
	"CancelOrganizationJoinRequest":  authenticated,
	"ResolveOrganizationJoinRequest": authenticated,

	"ListOrganizationInvitations": authenticated,
	"InviteOrganizationMember":    authenticated,
	"ListContestInvitations":      authenticated,
	"InviteContestParticipant":    authenticated,
	"CancelInvitation":            authenticated,
	"RespondInvitation":           authenticated,
	"GetTeamJoinLink":             authenticated,
	"ResetTeamJoinLink":           authenticated,
	"DisableTeamJoinLink":         authenticated,
	"GetContestJoinLink":          authenticated,
	"ResetContestJoinLink":        authenticated,
	"DisableContestJoinLink":      authenticated,
	"GetJoinLinkTarget":           authenticated,
	"FollowJoinLink":              authenticated,

	"ListNotifications":           authenticated,
	"GetUnreadNotificationsCount": authenticated,
	"MarkNotificationAsRead":      authenticated,
	"MarkAllNotificationsAsRead":  authenticated,

	"ListTeams":            authenticated,
	"CreateTeam":           authenticated,
	"GetTeam":              authenticated,
	"UpdateTeam":           authenticated,
	"DeleteTeam":           authenticated,
	"ListTeamMembers":      authenticated,
	"AddTeamMember":        authenticated,
	"UpdateTeamMemberRole": authenticated,
	"RemoveTeamMember":     authenticated,

	"CreateProblem":         authenticated,
	"ListProblemTemplates":  authenticated,
	"ListUserSubmissions":   authenticated,
	"GetSubmission":         authenticated,
	"UploadAvatar":          authenticated,
	"DeleteAvatar":          authenticated,
	"UpdateUser":            authenticated,
	"ClaimTemporaryUser":    authenticated,
	"ListMyClaimedAccounts": authenticated,

	"CreatePost":              admin,
	"PatchPostById":           admin,
	"DeletePostById":          admin,
	"ListSubmissions":         admin,
	"AdminChangeEmail":        admin,
	"AdminSetPassword":        admin,
	"AdminSendPasswordReset":  admin,
	"AdminResendVerification": admin,

	"GetContest":                   public,
	"DownloadContestStatementsPdf": public,
	"ListContestBooklets":          public,
	"GetContestScoreboard":         public,
	"UpdateContest":                authenticated,
	"DeleteContest":                authenticated,
	"CreateContestProblem":         authenticated,
	"GetContestProblem":            public,
	"GetContestProblemAsset":       public,
	"DeleteContestProblem":         authenticated,
	"ReorderContestProblems":       authenticated,
	"UpdateContestMember":          authenticated,
	"DeleteContestMember":          authenticated,
	"ListContestMembers":           authenticated,
	"ListContestTeams":             authenticated,
	"CreateContestTeam":            authenticated,
	"UpdateContestTeam":            authenticated,
	"DeleteContestTeam":            authenticated,
	"ListContestSubmissions":       authenticated,
	"CreateSubmission":             authenticated,
	"ListContestDrafts":            authenticated,
	"CreateContestDraft":           authenticated,
	"StartVirtualRun":              authenticated,
	"FinishVirtualRun":             authenticated,
	"DeleteContestDraft":           authenticated,
	"RejudgeContest":               authenticated,
	"BlockProblemForUser":          authenticated,
	"UnblockProblemForUser":        authenticated,
	"GetProblemBlockStatusForUser": authenticated,
	"ListContestJoinRequests":      authenticated,
	"CreateContestJoinRequest":     authenticated,
	"CancelContestJoinRequest":     authenticated,
	"ResolveContestJoinRequest":    authenticated,
	"UpdateSubmission":             authenticated,

	"ListContestAnnouncements":   public,
	"CreateContestAnnouncement":  authenticated,
	"DeleteContestAnnouncement":  authenticated,
	"ListContestClarifications":  authenticated,
	"CreateContestClarification": authenticated,
	"AnswerContestClarification": authenticated,

	"GetProblemDraft":        authenticated,
	"SaveProblemDraft":       authenticated,
	"BuildProblemPackage":    authenticated,
	"ListProblemBuilds":      authenticated,
	"GetProblemBuild":        authenticated,
	"CancelProblemBuild":     authenticated,
	"UploadProblemDraft":     authenticated,
	"GetProblemDraftObject":  authenticated,
	"ExportProblemDraft":     authenticated,
	"GetProblem":             public,
	"GetProblemAsset":        public,
	"UpdateProblem":          authenticated,
	"DeleteProblem":          authenticated,
	"ImportProblem":          authenticated,
	"DownloadProblemPackage": authenticated,
	"ListProblemPackages":    authenticated,
	"ListProblemMembers":     authenticated,
	"CreateProblemMember":    authenticated,
	"UpdateProblemMember":    authenticated,
	"DeleteProblemMember":    authenticated,
}

func authorizeOperation(ctx context.Context, operationID string) error {
	policy, ok := endpointPolicies[operationID]
	if !ok {
		return pkg.Wrap(pkg.ErrForbidden, nil, "endpoint not permitted by authorization policy")
	}
	switch policy {
	case authenticated:
		_, err := accessmodel.RequireUser(ctx)
		return err
	case admin:
		return accessmodel.RequireAdmin(ctx)
	}
	return nil
}

// Rejects callers before the request body is read.
func authorizeBeforeBody(ctx context.Context, operationID string) error {
	return authorizeOperation(ctx, operationID)
}

func AuthzMiddleware() corev1.Middleware {
	return func(req ogenMiddleware.Request, next func(req ogenMiddleware.Request) (ogenMiddleware.Response, error)) (ogenMiddleware.Response, error) {
		if err := authorizeOperation(req.Context, req.OperationID); err != nil {
			return ogenMiddleware.Response{}, err
		}
		return next(req)
	}
}
