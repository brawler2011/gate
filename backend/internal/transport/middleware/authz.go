package middleware

import (
	"context"
	"fmt"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	ogenMiddleware "github.com/ogen-go/ogen/middleware"
)

type EvalContext struct {
	User    accessmodel.User
	Request ogenMiddleware.Request
}

type AccessEvaluator func(ctx context.Context, evalCtx *EvalContext) error

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
		"CreateContest":      authenticatedPolicy(),
		"GetMyContestRole":   authenticatedPolicy(),

		"ListOrganizations":              authenticatedPolicy(),
		"CreateOrganization":             authenticatedPolicy(),
		"GetOrganization":                authenticatedPolicy(),
		"UpdateOrganization":             authenticatedPolicy(),
		"DeleteOrganization":             authenticatedPolicy(),
		"ListOrganizationMembers":        authenticatedPolicy(),
		"AddOrganizationMember":          authenticatedPolicy(),
		"RemoveOrganizationMember":       authenticatedPolicy(),
		"BatchCreateOrganizationUsers":   authenticatedPolicy(),
		"ListOrganizationContests":       policy(Public),
		"ListOrganizationInvitations":    authenticatedPolicy(),
		"InviteOrganizationMember":       authenticatedPolicy(),
		"CancelOrganizationInvitation":   authenticatedPolicy(),
		"AcceptOrganizationInvitation":   authenticatedPolicy(),
		"DeclineOrganizationInvitation":  authenticatedPolicy(),
		"ListOrganizationJoinRequests":   authenticatedPolicy(),
		"CreateOrganizationJoinRequest":  authenticatedPolicy(),
		"GetMyOrganizationJoinRequest":   authenticatedPolicy(),
		"CancelOrganizationJoinRequest":  authenticatedPolicy(),
		"ApproveOrganizationJoinRequest": authenticatedPolicy(),
		"RejectOrganizationJoinRequest":  authenticatedPolicy(),

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
		"ListUserContests":      authenticatedPolicy(RequireSelfOrAdmin()),
		"ListUserSubmissions":   authenticatedPolicy(),
		"ListWorkshopContests":  authenticatedPolicy(),
		"GetSubmission":         authenticatedPolicy(),
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
		"DownloadContestStatementsPdf": policy(Public),
		"GetContestScoreboard":         policy(Public),
		"UpdateContest":                authenticatedPolicy(),
		"DeleteContest":                authenticatedPolicy(),
		"CreateContestProblem":         authenticatedPolicy(),
		"GetContestProblem":            policy(Public),
		"DeleteContestProblem":         authenticatedPolicy(),
		"ReorderContestProblems":       authenticatedPolicy(),
		"CreateContestMember":          authenticatedPolicy(),
		"UpdateContestMember":          authenticatedPolicy(),
		"DeleteContestMember":          authenticatedPolicy(),
		"ListContestMembers":           authenticatedPolicy(),
		"ListContestTeams":             authenticatedPolicy(),
		"CreateContestTeam":            authenticatedPolicy(),
		"UpdateContestTeam":            authenticatedPolicy(),
		"DeleteContestTeam":            authenticatedPolicy(),
		"ListContestSubmissions":       authenticatedPolicy(),
		"CreateSubmission":             authenticatedPolicy(),
		"ListContestDrafts":            authenticatedPolicy(),
		"CreateContestDraft":           authenticatedPolicy(),
		"DeleteContestDraft":           authenticatedPolicy(),
		"RejudgeSubmission":            authenticatedPolicy(),
		"RejudgeContestProblem":        authenticatedPolicy(),
		"RejudgeContest":               authenticatedPolicy(),
		"BlockSubmission":              authenticatedPolicy(),
		"UnblockSubmission":            authenticatedPolicy(),
		"BlockProblemForUser":          authenticatedPolicy(),
		"UnblockProblemForUser":        authenticatedPolicy(),
		"GetProblemBlockStatusForUser": authenticatedPolicy(),
		"ListContestJoinRequests":      authenticatedPolicy(),
		"CreateContestJoinRequest":     authenticatedPolicy(),
		"GetMyContestJoinRequest":      authenticatedPolicy(),
		"CancelContestJoinRequest":     authenticatedPolicy(),
		"ApproveContestJoinRequest":    authenticatedPolicy(),
		"RejectContestJoinRequest":     authenticatedPolicy(),

		"ListContestAnnouncements":   policy(Public),
		"CreateContestAnnouncement":  authenticatedPolicy(),
		"DeleteContestAnnouncement":  authenticatedPolicy(),
		"ListContestClarifications":  authenticatedPolicy(),
		"CreateContestClarification": authenticatedPolicy(),
		"AnswerContestClarification": authenticatedPolicy(),

		"GetProblem":          policy(Public),
		"GetProblemLimits":    policy(Public),
		"GetProblemStatement": authenticatedPolicy(),
		"UpdateProblem":       authenticatedPolicy(),
		"DeleteProblem":       authenticatedPolicy(),
		"ImportProblem":       authenticatedPolicy(),
		"PublishProblem":      authenticatedPolicy(),
		"ListProblemPackages": authenticatedPolicy(),
		"ListProblemMembers":  authenticatedPolicy(),
		"CreateProblemMember": authenticatedPolicy(),
		"UpdateProblemMember": authenticatedPolicy(),
		"DeleteProblemMember": authenticatedPolicy(),
		"ListProblemTeams":    authenticatedPolicy(),
		"CreateProblemTeam":   authenticatedPolicy(),
		"UpdateProblemTeam":   authenticatedPolicy(),
		"DeleteProblemTeam":   authenticatedPolicy(),
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
		policies[operationID] = authenticatedPolicy()
	}

	return policies
}

// authorizeBeforeBody rejects missing identities and admin access without decoding
// request bodies. Self-or-admin checks use ogen's decoded username parameter.
func authorizeBeforeBody(ctx context.Context, operationID string) error {
	policy, ok := endpointPolicies[operationID]
	if !ok || len(policy.evaluators) == 0 {
		return pkg.Wrap(pkg.NoPermission, nil, "endpoint not permitted by authorization policy")
	}
	evalCtx := &EvalContext{User: accessmodel.UserFromContext(ctx)}
	for _, evaluator := range policy.beforeBody {
		if err := evaluator(ctx, evalCtx); err != nil {
			return err
		}
	}
	return nil
}

// AuthzMiddleware validates operation access before handlers are called.
func AuthzMiddleware() corev1.Middleware {
	return func(req ogenMiddleware.Request, next func(req ogenMiddleware.Request) (ogenMiddleware.Response, error)) (ogenMiddleware.Response, error) {
		operationID := req.OperationID
		policy, ok := endpointPolicies[operationID]
		if !ok || len(policy.evaluators) == 0 {
			// Zero-Trust Default Deny
			return ogenMiddleware.Response{}, pkg.Wrap(pkg.NoPermission, nil, "endpoint not permitted by authorization policy")
		}

		user := accessmodel.UserFromContext(req.Context)
		evalCtx := &EvalContext{
			User:    user,
			Request: req,
		}

		for _, evaluator := range policy.evaluators {
			if err := evaluator(req.Context, evalCtx); err != nil {
				return ogenMiddleware.Response{}, err
			}
		}

		return next(req)
	}
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
