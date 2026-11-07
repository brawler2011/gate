package middleware

import (
	"context"
	"fmt"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	ogenMiddleware "github.com/ogen-go/ogen/middleware"
)

type EvalContext struct {
	User        accessmodel.User
	Request     ogenMiddleware.Request
	OperationID string
	Deps        strictAuthzDependencies
}

type AccessEvaluator func(ctx context.Context, evalCtx *EvalContext) error

type strictAuthzDependencies struct {
	permissionsUC     PermissionsUC
	submissionsUC     SubmissionsUC
	organizationsRepo OrganizationsRepo
	contestsRepo      ContestsRepo
}

// Parameterless Evaluators (Bare Function Pointers)
func Public(ctx context.Context, evalCtx *EvalContext) error {
	return nil
}

func RequireAuth(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.User.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	return nil
}

func RequireAdmin(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.User.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	if !evalCtx.User.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "admin access required")
	}
	return nil
}

// Parameterized Evaluator Factories (Closure Factories)
func RequireSelfOrAdmin() AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		if evalCtx.User.IsAdmin() {
			return nil
		}
		targetUsername, err := pathString(evalCtx.Request, "username")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "user identifier is required for authorization")
		}
		cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
		if cleanTarget != strings.ToLower(evalCtx.User.Username) {
			return pkg.Wrap(pkg.NoPermission, nil, "can only modify your own resource")
		}
		return nil
	}
}

func RequireOrgPermission(action accessmodel.OrgAction) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		targetLogin, err := pathString(evalCtx.Request, "login", "org_login")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required for authorization")
		}
		if evalCtx.Deps.organizationsRepo == nil {
			return pkg.Wrap(pkg.ErrInternal, nil, "organizations repository dependency is missing")
		}
		org, err := evalCtx.Deps.organizationsRepo.GetOrganizationByLogin(ctx, targetLogin)
		if err != nil {
			return pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
		}
		allowed, err := evalCtx.Deps.permissionsUC.HasOrganizationPermission(ctx, org.ID, evalCtx.User.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient organization permissions")
		}
		return nil
	}
}

func RequireContestPermission(action accessmodel.ContestAction) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() && action != accessmodel.ActionGetContest && action != accessmodel.ActionGetContestProblem {
			return pkg.ErrUnauthenticated
		}
		orgLogin, contestLogin, err := contestLogins(evalCtx.Request)
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "organization and contest logins are required for authorization")
		}
		if evalCtx.Deps.contestsRepo == nil {
			return pkg.Wrap(pkg.ErrInternal, nil, "contests repository dependency is missing")
		}
		contest, err := evalCtx.Deps.contestsRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
		if err != nil {
			return pkg.Wrap(pkg.ErrNotFound, err, "contest not found")
		}
		allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contest.ID, evalCtx.User.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			if evalCtx.User.IsGuest() {
				return pkg.ErrUnauthenticated
			}
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient contest permissions")
		}
		return nil
	}
}

func RequireProblemPermission(action accessmodel.ProblemAction) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		problemID, err := pathUUID(evalCtx.Request, "problemId", "id")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "problem id is required for authorization")
		}
		allowed, err := evalCtx.Deps.permissionsUC.HasProblemPermission(ctx, problemID, evalCtx.User.Id, action)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient problem permissions")
		}
		return nil
	}
}

var endpointPolicies = buildEndpointPolicies()

// endpointPolicy keeps body-independent identity checks alongside the full policy.
type endpointPolicy struct {
	beforeBody []AccessEvaluator
	evaluators []AccessEvaluator
}

func policy(evaluators ...AccessEvaluator) endpointPolicy {
	return endpointPolicy{evaluators: evaluators}
}

func authenticatedPolicy(evaluators ...AccessEvaluator) endpointPolicy {
	return endpointPolicy{
		beforeBody: []AccessEvaluator{RequireAuth},
		evaluators: append([]AccessEvaluator{RequireAuth}, evaluators...),
	}
}

func adminPolicy() endpointPolicy {
	return endpointPolicy{
		beforeBody: []AccessEvaluator{RequireAuth, RequireAdmin},
		evaluators: []AccessEvaluator{RequireAuth, RequireAdmin},
	}
}

func buildEndpointPolicies() map[string]endpointPolicy {
	policies := map[string]endpointPolicy{
		"GetHealth":           policy(Public),
		"ListPublicContests":  policy(Public),
		"ListPosts":           policy(Public),
		"GetPostById":         policy(Public),
		"GetPostImage":        policy(Public),
		"GetPublishedPackage": policy(Public),
		"ListProblems":        policy(Public),
		"ListUsers":           authenticatedPolicy(),
		"GetUser":             policy(Public),
		"GetUserAvatar":       policy(Public),
		"GetLanguages":        policy(Public),
		"Register":            policy(Public),
		"Login":               policy(Public),
		"Logout":              policy(Public),
		"VerifyEmail":         policy(Public),
		"ResendVerification":  policy(Public),
		"ForgotPassword":      policy(Public),
		"ResetPassword":       policy(Public),
		"ConfirmEmailChange":  policy(Public),

		"GetMe":              authenticatedPolicy(),
		"GetMyDashboard":     authenticatedPolicy(),
		"ChangePassword":     authenticatedPolicy(),
		"RequestEmailChange": authenticatedPolicy(),
		"CreateContest":      authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"GetMyContestRole":   authenticatedPolicy(),

		"ListOrganizations":              authenticatedPolicy(),
		"CreateOrganization":             authenticatedPolicy(),
		"GetOrganization":                authenticatedPolicy(RequireOrgPermission(accessmodel.ActionViewOrganization)),
		"UpdateOrganization":             authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"DeleteOrganization":             authenticatedPolicy(RequireOrgPermission(accessmodel.ActionDeleteOrganization)),
		"ListOrganizationMembers":        authenticatedPolicy(RequireOrgPermission(accessmodel.ActionViewOrganization)),
		"AddOrganizationMember":          authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"RemoveOrganizationMember":       authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"BatchCreateOrganizationUsers":   authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"ListOrganizationContests":       policy(Public),
		"ListOrganizationInvitations":    authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"InviteOrganizationMember":       authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"CancelOrganizationInvitation":   authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"AcceptOrganizationInvitation":   authenticatedPolicy(),
		"DeclineOrganizationInvitation":  authenticatedPolicy(),
		"ListOrganizationJoinRequests":   authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"CreateOrganizationJoinRequest":  authenticatedPolicy(),
		"GetMyOrganizationJoinRequest":   authenticatedPolicy(),
		"CancelOrganizationJoinRequest":  authenticatedPolicy(),
		"ApproveOrganizationJoinRequest": authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),
		"RejectOrganizationJoinRequest":  authenticatedPolicy(RequireOrgPermission(accessmodel.ActionManageOrganization)),

		"ListNotifications":           authenticatedPolicy(),
		"GetUnreadNotificationsCount": authenticatedPolicy(),
		"MarkNotificationAsRead":      authenticatedPolicy(),
		"MarkAllNotificationsAsRead":  authenticatedPolicy(),

		"ListTeams":            authenticatedPolicy(),
		"CreateTeam":           authenticatedPolicy(),
		"GetTeam":              authenticatedPolicy(),
		"UpdateTeam":           authenticatedPolicy(),
		"DeleteTeam":           authenticatedPolicy(),
		"ListTeamMembers":      authenticatedPolicy(),
		"AddTeamMember":        authenticatedPolicy(),
		"UpdateTeamMemberRole": authenticatedPolicy(),
		"RemoveTeamMember":     authenticatedPolicy(),
		"ListTeamContests":     authenticatedPolicy(),
		"ListTeamProblems":     authenticatedPolicy(),

		"CreateProblem":         authenticatedPolicy(),
		"ListProblemTemplates":  authenticatedPolicy(),
		"ListUserContests":      authenticatedPolicy(checkListUserContestsAccess),
		"ListUserSubmissions":   authenticatedPolicy(checkListUserSubmissionsAccess),
		"ListWorkshopContests":  authenticatedPolicy(),
		"GetSubmission":         authenticatedPolicy(checkGetSubmissionAccess),
		"UploadAvatar":          authenticatedPolicy(RequireSelfOrAdmin()),
		"DeleteAvatar":          authenticatedPolicy(RequireSelfOrAdmin()),
		"UpdateUser":            authenticatedPolicy(RequireSelfOrAdmin()),
		"ClaimTemporaryUser":    authenticatedPolicy(),
		"ListMyClaimedAccounts": authenticatedPolicy(),

		"ListAdminContests":       adminPolicy(),
		"CreatePost":              adminPolicy(),
		"PatchPostById":           adminPolicy(),
		"DeletePostById":          adminPolicy(),
		"ListSubmissions":         adminPolicy(),
		"AdminChangeEmail":        adminPolicy(),
		"AdminSetPassword":        adminPolicy(),
		"AdminSendPasswordReset":  adminPolicy(),
		"AdminResendVerification": adminPolicy(),

		"GetContest":                   policy(Public),
		"DownloadContestStatementsPdf": policy(RequireContestPermission(accessmodel.ActionGetContest)),
		"GetContestScoreboard":         policy(RequireContestPermission(accessmodel.ActionGetMonitor)),
		"UpdateContest":                authenticatedPolicy(RequireContestPermission(accessmodel.ActionUpdateContest)),
		"DeleteContest":                authenticatedPolicy(RequireContestPermission(accessmodel.ActionAdminContest)),
		"CreateContestProblem":         authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"GetContestProblem":            policy(RequireContestPermission(accessmodel.ActionGetContestProblem)),
		"DeleteContestProblem":         authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"ReorderContestProblems":       authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"CreateContestMember":          authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"UpdateContestMember":          authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"DeleteContestMember":          authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"ListContestMembers":           authenticatedPolicy(checkListContestMembersAccess),
		"ListContestTeams":             authenticatedPolicy(checkListContestMembersAccess),
		"CreateContestTeam":            authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"UpdateContestTeam":            authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"DeleteContestTeam":            authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"ListContestSubmissions":       authenticatedPolicy(checkListContestSubmissionsAccess),
		"CreateSubmission":             authenticatedPolicy(RequireContestPermission(accessmodel.ActionCreateSubmission)),
		"ListContestDrafts":            authenticatedPolicy(checkListContestDraftsAccess),
		"CreateContestDraft":           authenticatedPolicy(RequireContestPermission(accessmodel.ActionGetContest)),
		"DeleteContestDraft":           authenticatedPolicy(RequireContestPermission(accessmodel.ActionGetContest)),
		"RejudgeSubmission":            authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"RejudgeContestProblem":        authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"RejudgeContest":               authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"BlockSubmission":              authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"UnblockSubmission":            authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"BlockProblemForUser":          authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"UnblockProblemForUser":        authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"GetProblemBlockStatusForUser": authenticatedPolicy(checkGetProblemBlockStatusAccess),
		"ListContestJoinRequests":      authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"CreateContestJoinRequest":     authenticatedPolicy(),
		"GetMyContestJoinRequest":      authenticatedPolicy(),
		"CancelContestJoinRequest":     authenticatedPolicy(),
		"ApproveContestJoinRequest":    authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"RejectContestJoinRequest":     authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),

		"ListContestAnnouncements":   policy(RequireContestPermission(accessmodel.ActionGetContest)),
		"CreateContestAnnouncement":  authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"DeleteContestAnnouncement":  authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),
		"ListContestClarifications":  authenticatedPolicy(RequireContestPermission(accessmodel.ActionGetContest)),
		"CreateContestClarification": authenticatedPolicy(RequireContestPermission(accessmodel.ActionGetContest)),
		"AnswerContestClarification": authenticatedPolicy(RequireContestPermission(accessmodel.ActionManageContest)),

		"GetProblem":          policy(Public),
		"GetProblemLimits":    policy(Public),
		"GetProblemStatement": authenticatedPolicy(RequireProblemPermission(accessmodel.ActionViewProblem)),
		"UpdateProblem":       authenticatedPolicy(RequireProblemPermission(accessmodel.ActionEditProblem)),
		"DeleteProblem":       authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"ImportProblem":       authenticatedPolicy(RequireProblemPermission(accessmodel.ActionEditProblem)),
		"PublishProblem":      authenticatedPolicy(RequireProblemPermission(accessmodel.ActionEditProblem)),
		"ListProblemPackages": authenticatedPolicy(),
		"ListProblemMembers":  authenticatedPolicy(),
		"CreateProblemMember": authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"UpdateProblemMember": authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"DeleteProblemMember": authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"ListProblemTeams":    authenticatedPolicy(),
		"CreateProblemTeam":   authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"UpdateProblemTeam":   authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
		"DeleteProblemTeam":   authenticatedPolicy(RequireProblemPermission(accessmodel.ActionAdminProblem)),
	}

	for _, operationID := range []string{
		"UpdateProblemLimits",
		"UpdateProblemStatement",
		"ListProblemCheckers",
		"CreateProblemChecker",
		"GetProblemChecker",
		"UpdateProblemChecker",
		"DeleteProblemChecker",
		"ListProblemGenerators",
		"CreateProblemGenerator",
		"GetProblemGenerator",
		"UpdateProblemGenerator",
		"DeleteProblemGenerator",
		"ListProblemInteractors",
		"CreateProblemInteractor",
		"GetProblemInteractor",
		"UpdateProblemInteractor",
		"DeleteProblemInteractor",
		"ListProblemValidators",
		"CreateProblemValidator",
		"GetProblemValidator",
		"UpdateProblemValidator",
		"DeleteProblemValidator",
		"ListProblemLibs",
		"CreateProblemLib",
		"GetProblemLib",
		"UpdateProblemLib",
		"DeleteProblemLib",
		"ListProblemMediaFiles",
		"CreateProblemMediaFile",
		"GetProblemMediaFile",
		"UpdateProblemMediaFile",
		"DeleteProblemMediaFile",
		"ListProblemWorkshopSubmissions",
		"CreateProblemWorkshopSubmission",
		"GetProblemWorkshopSubmission",
		"UpdateProblemWorkshopSubmission",
		"DeleteProblemWorkshopSubmission",
		"ListProblemTests",
		"CreateProblemTestFile",
		"GetProblemTestFile",
		"UpdateProblemTestFile",
		"DeleteProblemTestFile",
		"UpdateProblemTestsConfig",
		"CompileProblemComponent",
		"GenerateTests",
		"ValidateAllTests",
		"TestSolution",
	} {
		policies[operationID] = authenticatedPolicy(RequireProblemPermission(accessmodel.ActionEditProblem))
	}

	return policies
}

// authorizeBeforeBody rejects missing identities and admin access without decoding
// request bodies. Resource-specific checks still use ogen's decoded parameters.
func authorizeBeforeBody(ctx context.Context, operationID string) error {
	policy, ok := endpointPolicies[operationID]
	if !ok || len(policy.evaluators) == 0 {
		return pkg.Wrap(pkg.NoPermission, nil, "endpoint not permitted by authorization policy")
	}
	evalCtx := &EvalContext{User: accessmodel.UserFromContext(ctx), OperationID: operationID}
	for _, evaluator := range policy.beforeBody {
		if err := evaluator(ctx, evalCtx); err != nil {
			return err
		}
	}
	return nil
}

// AuthzMiddleware validates operation access before handlers are called.
func AuthzMiddleware(permissionsUC PermissionsUC, submissionsUC SubmissionsUC, organizationsRepo OrganizationsRepo, contestsRepo ContestsRepo) corev1.Middleware {
	deps := strictAuthzDependencies{
		permissionsUC:     permissionsUC,
		submissionsUC:     submissionsUC,
		organizationsRepo: organizationsRepo,
		contestsRepo:      contestsRepo,
	}

	return func(req ogenMiddleware.Request, next func(req ogenMiddleware.Request) (ogenMiddleware.Response, error)) (ogenMiddleware.Response, error) {
		operationID := req.OperationID
		policy, ok := endpointPolicies[operationID]
		if !ok || len(policy.evaluators) == 0 {
			// Zero-Trust Default Deny
			return ogenMiddleware.Response{}, pkg.Wrap(pkg.NoPermission, nil, "endpoint not permitted by authorization policy")
		}

		user := accessmodel.UserFromContext(req.Context)
		evalCtx := &EvalContext{
			User:        user,
			Request:     req,
			OperationID: operationID,
			Deps:        deps,
		}

		for _, evaluator := range policy.evaluators {
			if err := evaluator(req.Context, evalCtx); err != nil {
				return ogenMiddleware.Response{}, err
			}
		}

		return next(req)
	}
}

func checkListUserContestsAccess(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.User.IsAdmin() {
		return nil
	}
	targetUsername, err := pathString(evalCtx.Request, "username")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user identifier is required for authorization")
	}
	cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
	if cleanTarget != strings.ToLower(evalCtx.User.Username) {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	return nil
}

func checkListUserSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	targetUsername, err := pathString(evalCtx.Request, "username")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid submissions request")
	}

	cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
	isSelf := cleanTarget == strings.ToLower(evalCtx.User.Username)

	if !isSelf && !evalCtx.User.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
	}

	if isSelf {
		if contestID, err := queryUUID(evalCtx.Request, "contestId"); err == nil && contestID != uuid.Nil {
			allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(
				ctx,
				contestID,
				evalCtx.User.Id,
				accessmodel.ActionListOwnSubmissions,
			)
			if err != nil {
				return err
			}
			if !allowed {
				return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
			}
		}
	}

	return nil
}

func checkListContestMembersAccess(ctx context.Context, evalCtx *EvalContext) error {
	contestLogin, err := pathString(evalCtx.Request, "contest_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "contest login is required for authorization")
	}
	orgLogin, err := pathString(evalCtx.Request, "org_login")
	if err != nil || orgLogin == "" {
		return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required for contest authorization")
	}
	if evalCtx.Deps.contestsRepo == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "contests repository dependency is missing")
	}
	contest, err := evalCtx.Deps.contestsRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return pkg.Wrap(pkg.ErrNotFound, err, "contest not found")
	}
	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contest.ID, evalCtx.User.Id, accessmodel.ActionGetMonitor)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	allowed, err = evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contest.ID, evalCtx.User.Id, accessmodel.ActionListOwnSubmissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
}

func checkListContestSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	orgLogin, err := pathString(evalCtx.Request, "org_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required")
	}
	contestLogin, err := pathString(evalCtx.Request, "contest_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "contest login is required")
	}

	if evalCtx.Deps.contestsRepo == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "contests repository dependency is missing")
	}
	contest, err := evalCtx.Deps.contestsRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return pkg.Wrap(pkg.ErrNotFound, err, "contest not found")
	}

	action := accessmodel.ActionListUsersSubmissions
	errMessage := "insufficient permission to list all contest submissions"

	if targetUserID, err := queryUUID(evalCtx.Request, "userId"); err == nil && targetUserID != uuid.Nil {
		if targetUserID == evalCtx.User.Id {
			action = accessmodel.ActionListOwnSubmissions
			errMessage = "insufficient permission to view own submissions in this contest"
		} else {
			action = accessmodel.ActionListUsersSubmissions
			errMessage = "insufficient permission to view other users' submissions"
		}
	}

	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contest.ID, evalCtx.User.Id, action)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.Wrap(pkg.NoPermission, nil, errMessage)
	}

	return nil
}

func checkListContestDraftsAccess(ctx context.Context, evalCtx *EvalContext) error {
	orgLogin, err := pathString(evalCtx.Request, "org_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required")
	}
	contestLogin, err := pathString(evalCtx.Request, "contest_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "contest login is required")
	}

	if evalCtx.Deps.contestsRepo == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "contests repository dependency is missing")
	}
	contest, err := evalCtx.Deps.contestsRepo.GetContestByOrgLoginAndContestLogin(ctx, orgLogin, contestLogin)
	if err != nil {
		return pkg.Wrap(pkg.ErrNotFound, err, "contest not found")
	}

	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contest.ID, evalCtx.User.Id, accessmodel.ActionGetContest)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	return nil
}

func checkGetSubmissionAccess(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.Deps.submissionsUC == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "submissions authorization dependency is not configured")
	}

	submissionID, err := pathUUID(evalCtx.Request, "submission_id")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "submission id is required for authorization")
	}

	submission, err := evalCtx.Deps.submissionsUC.GetSubmission(ctx, submissionID)
	if err != nil {
		return err
	}

	if evalCtx.User.IsAdmin() {
		return nil
	}

	if submission.CreatedBy != nil && *submission.CreatedBy == evalCtx.User.Id {
		return nil
	}

	if submission.ContestID != nil {
		allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(
			ctx,
			*submission.ContestID,
			evalCtx.User.Id,
			accessmodel.ActionGetOtherUserSubmission,
		)
		if err == nil && allowed {
			return nil
		}
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
}

func checkGetProblemBlockStatusAccess(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.User.IsAdmin() {
		return nil
	}

	targetUserID, err := pathUUID(evalCtx.Request, "user_id")
	if err == nil && targetUserID == evalCtx.User.Id {
		return nil
	}

	return RequireContestPermission(accessmodel.ActionManageContest)(ctx, evalCtx)
}

func pathString(req ogenMiddleware.Request, names ...string) (string, error) {
	for _, name := range names {
		value, _ := req.Params.Path(name)
		if value, ok := value.(string); ok && value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("missing string path parameter %q", names)
}

func pathUUID(req ogenMiddleware.Request, names ...string) (uuid.UUID, error) {
	for _, name := range names {
		value, _ := req.Params.Path(name)
		if id, ok := value.(uuid.UUID); ok && id != uuid.Nil {
			return id, nil
		}
	}
	return uuid.Nil, fmt.Errorf("missing UUID path parameter %q", names)
}

func queryUUID(req ogenMiddleware.Request, name string) (uuid.UUID, error) {
	value, _ := req.Params.Query(name)
	if value, ok := value.(corev1.OptUUID); ok && value.IsSet() && value.Value != uuid.Nil {
		return value.Value, nil
	}
	return uuid.Nil, fmt.Errorf("missing UUID query parameter %q", name)
}

// CreateSubmission carries contest logins in the query; contest routes use path parameters.
func contestLogins(req ogenMiddleware.Request) (string, string, error) {
	if req.OperationID == "CreateSubmission" {
		orgValue, _ := req.Params.Query("organization_login")
		contestValue, _ := req.Params.Query("contest_login")
		orgLogin, orgOK := orgValue.(string)
		contestLogin, contestOK := contestValue.(string)
		if !orgOK || !contestOK || orgLogin == "" || contestLogin == "" {
			return "", "", fmt.Errorf("missing organization or contest query parameter")
		}
		return orgLogin, contestLogin, nil
	}
	orgLogin, err := pathString(req, "org_login")
	if err != nil {
		return "", "", err
	}
	contestLogin, err := pathString(req, "contest_login")
	return orgLogin, contestLogin, err
}
