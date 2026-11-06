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

func buildEndpointPolicies() map[string][]AccessEvaluator {
	policies := map[string][]AccessEvaluator{
		"GetHealth":           {Public},
		"ListPublicContests":  {Public},
		"ListPosts":           {Public},
		"GetPostById":         {Public},
		"GetPostImage":        {Public},
		"GetPublishedPackage": {Public},
		"ListProblems":        {Public},
		"ListUsers":           {RequireAuth},
		"GetUser":             {Public},
		"GetUserAvatar":       {Public},
		"GetLanguages":        {Public},
		"Register":            {Public},
		"Login":               {Public},
		"Logout":              {Public},
		"VerifyEmail":         {Public},
		"ResendVerification":  {Public},
		"ForgotPassword":      {Public},
		"ResetPassword":       {Public},
		"ConfirmEmailChange":  {Public},

		"GetMe":              {RequireAuth},
		"GetMyDashboard":     {RequireAuth},
		"ChangePassword":     {RequireAuth},
		"RequestEmailChange": {RequireAuth},
		"CreateContest":      {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"GetMyContestRole":   {RequireAuth},

		"ListOrganizations":              {RequireAuth},
		"CreateOrganization":             {RequireAuth},
		"GetOrganization":                {RequireAuth, RequireOrgPermission(accessmodel.ActionViewOrganization)},
		"UpdateOrganization":             {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"DeleteOrganization":             {RequireAuth, RequireOrgPermission(accessmodel.ActionDeleteOrganization)},
		"ListOrganizationMembers":        {RequireAuth, RequireOrgPermission(accessmodel.ActionViewOrganization)},
		"AddOrganizationMember":          {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"RemoveOrganizationMember":       {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"BatchCreateOrganizationUsers":   {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"ListOrganizationContests":       {RequireAuth, RequireOrgPermission(accessmodel.ActionViewOrganization)},
		"ListOrganizationInvitations":    {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"InviteOrganizationMember":       {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"CancelOrganizationInvitation":   {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"AcceptOrganizationInvitation":   {RequireAuth},
		"DeclineOrganizationInvitation":  {RequireAuth},
		"ListOrganizationJoinRequests":   {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"CreateOrganizationJoinRequest":  {RequireAuth},
		"GetMyOrganizationJoinRequest":   {RequireAuth},
		"CancelOrganizationJoinRequest":  {RequireAuth},
		"ApproveOrganizationJoinRequest": {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},
		"RejectOrganizationJoinRequest":  {RequireAuth, RequireOrgPermission(accessmodel.ActionManageOrganization)},

		"ListNotifications":           {RequireAuth},
		"GetUnreadNotificationsCount": {RequireAuth},
		"MarkNotificationAsRead":      {RequireAuth},
		"MarkAllNotificationsAsRead":  {RequireAuth},

		"ListTeams":            {RequireAuth},
		"CreateTeam":           {RequireAuth},
		"GetTeam":              {RequireAuth},
		"UpdateTeam":           {RequireAuth},
		"DeleteTeam":           {RequireAuth},
		"ListTeamMembers":      {RequireAuth},
		"AddTeamMember":        {RequireAuth},
		"UpdateTeamMemberRole": {RequireAuth},
		"RemoveTeamMember":     {RequireAuth},
		"ListTeamContests":     {RequireAuth},
		"ListTeamProblems":     {RequireAuth},

		"CreateProblem":         {RequireAuth},
		"ListProblemTemplates":  {RequireAuth},
		"ListUserContests":      {RequireAuth, checkListUserContestsAccess},
		"ListUserSubmissions":   {RequireAuth, checkListUserSubmissionsAccess},
		"ListWorkshopContests":  {RequireAuth},
		"GetSubmission":         {RequireAuth, checkGetSubmissionAccess},
		"UploadAvatar":          {RequireAuth, RequireSelfOrAdmin()},
		"DeleteAvatar":          {RequireAuth, RequireSelfOrAdmin()},
		"UpdateUser":            {RequireAuth, RequireSelfOrAdmin()},
		"ClaimTemporaryUser":    {RequireAuth},
		"ListMyClaimedAccounts": {RequireAuth},

		"ListAdminContests":       {RequireAuth, RequireAdmin},
		"CreatePost":              {RequireAuth, RequireAdmin},
		"PatchPostById":           {RequireAuth, RequireAdmin},
		"DeletePostById":          {RequireAuth, RequireAdmin},
		"ListSubmissions":         {RequireAuth, RequireAdmin},
		"AdminChangeEmail":        {RequireAuth, RequireAdmin},
		"AdminSetPassword":        {RequireAuth, RequireAdmin},
		"AdminSendPasswordReset":  {RequireAuth, RequireAdmin},
		"AdminResendVerification": {RequireAuth, RequireAdmin},

		"GetContest":                   {RequireContestPermission(accessmodel.ActionGetContest)},
		"DownloadContestStatementsPdf": {RequireContestPermission(accessmodel.ActionGetContest)},
		"GetContestScoreboard":         {RequireContestPermission(accessmodel.ActionGetMonitor)},
		"UpdateContest":                {RequireAuth, RequireContestPermission(accessmodel.ActionUpdateContest)},
		"DeleteContest":                {RequireAuth, RequireContestPermission(accessmodel.ActionAdminContest)},
		"CreateContestProblem":         {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"GetContestProblem":            {RequireContestPermission(accessmodel.ActionGetContestProblem)},
		"DeleteContestProblem":         {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"ReorderContestProblems":       {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"CreateContestMember":          {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"UpdateContestMember":          {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"DeleteContestMember":          {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"ListContestMembers":           {RequireAuth, checkListContestMembersAccess},
		"ListContestTeams":             {RequireAuth, checkListContestMembersAccess},
		"CreateContestTeam":            {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"UpdateContestTeam":            {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"DeleteContestTeam":            {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"ListContestSubmissions":       {RequireAuth, checkListContestSubmissionsAccess},
		"CreateSubmission":             {RequireAuth, RequireContestPermission(accessmodel.ActionCreateSubmission)},
		"ListContestDrafts":            {RequireAuth, checkListContestDraftsAccess},
		"CreateContestDraft":           {RequireAuth, RequireContestPermission(accessmodel.ActionGetContest)},
		"DeleteContestDraft":           {RequireAuth, RequireContestPermission(accessmodel.ActionGetContest)},
		"RejudgeSubmission":            {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"RejudgeContestProblem":        {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"RejudgeContest":               {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"BlockSubmission":              {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"UnblockSubmission":            {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"BlockProblemForUser":          {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"UnblockProblemForUser":        {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"GetProblemBlockStatusForUser": {RequireAuth, checkGetProblemBlockStatusAccess},
		"ListContestJoinRequests":      {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"CreateContestJoinRequest":     {RequireAuth},
		"GetMyContestJoinRequest":      {RequireAuth},
		"CancelContestJoinRequest":     {RequireAuth},
		"ApproveContestJoinRequest":    {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"RejectContestJoinRequest":     {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},

		"ListContestAnnouncements":   {RequireContestPermission(accessmodel.ActionGetContest)},
		"CreateContestAnnouncement":  {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"DeleteContestAnnouncement":  {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},
		"ListContestClarifications":  {RequireAuth, RequireContestPermission(accessmodel.ActionGetContest)},
		"CreateContestClarification": {RequireAuth, RequireContestPermission(accessmodel.ActionGetContest)},
		"AnswerContestClarification": {RequireAuth, RequireContestPermission(accessmodel.ActionManageContest)},

		"GetProblem":          {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"GetProblemLimits":    {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"GetProblemStatement": {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"UpdateProblem":       {RequireAuth, RequireProblemPermission(accessmodel.ActionEditProblem)},
		"DeleteProblem":       {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"ImportProblem":       {RequireAuth, RequireProblemPermission(accessmodel.ActionEditProblem)},
		"PublishProblem":      {RequireAuth, RequireProblemPermission(accessmodel.ActionEditProblem)},
		"ListProblemPackages": {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"ListProblemMembers":  {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"CreateProblemMember": {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"UpdateProblemMember": {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"DeleteProblemMember": {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"ListProblemTeams":    {RequireAuth, RequireProblemPermission(accessmodel.ActionViewProblem)},
		"CreateProblemTeam":   {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"UpdateProblemTeam":   {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
		"DeleteProblemTeam":   {RequireAuth, RequireProblemPermission(accessmodel.ActionAdminProblem)},
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
		policies[operationID] = []AccessEvaluator{RequireAuth, RequireProblemPermission(accessmodel.ActionEditProblem)}
	}

	return policies
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
		evaluators, ok := endpointPolicies[operationID]
		if !ok || len(evaluators) == 0 {
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

		for _, evaluator := range evaluators {
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
