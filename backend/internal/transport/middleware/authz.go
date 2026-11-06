package middleware

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	ogenMiddleware "github.com/ogen-go/ogen/middleware"
)

type EvalContext struct {
	User        accessmodel.User
	Request     interface{}
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
func RequireSelfOrAdmin(idFieldNames ...string) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		if evalCtx.User.IsAdmin() {
			return nil
		}
		if targetUsername, err := extractStringFromRequest(evalCtx.Request, idFieldNames...); err == nil && targetUsername != "" {
			cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
			if cleanTarget == strings.ToLower(evalCtx.User.Username) {
				return nil
			}
			return pkg.Wrap(pkg.NoPermission, nil, "can only modify your own resource")
		}
		targetUserID, err := extractUUIDFromRequest(evalCtx.Request, idFieldNames...)
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "user identifier is required for authorization")
		}
		if targetUserID != evalCtx.User.Id {
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
		var orgID uuid.UUID
		if targetLogin, err := extractStringFromRequest(evalCtx.Request, "Login", "OrgLogin"); err == nil && targetLogin != "" {
			if evalCtx.Deps.organizationsRepo == nil {
				return pkg.Wrap(pkg.ErrInternal, nil, "organizations repository dependency is missing")
			}
			org, err := evalCtx.Deps.organizationsRepo.GetOrganizationByLogin(ctx, targetLogin)
			if err != nil {
				return pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
			}
			orgID = org.ID
		} else {
			var err error
			orgID, err = extractUUIDFromRequest(evalCtx.Request, "OrganizationId", "OrgId", "Id")
			if err != nil {
				return pkg.Wrap(pkg.ErrBadInput, err, "organization identifier is required for authorization")
			}
		}
		allowed, err := evalCtx.Deps.permissionsUC.HasOrganizationPermission(ctx, orgID, evalCtx.User.Id, action)
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
		var contestID uuid.UUID
		if contestLogin, err := extractStringFromRequest(evalCtx.Request, "ContestLogin"); err == nil && contestLogin != "" {
			orgLogin, err := extractStringFromRequest(evalCtx.Request, "OrgLogin", "OrganizationLogin")
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
			contestID = contest.ID
		} else {
			var err error
			contestID, err = extractUUIDFromRequest(evalCtx.Request, "ContestId", "Id")
			if err != nil {
				return pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
			}
		}

		allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contestID, evalCtx.User.Id, action)
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
		problemID, err := extractUUIDFromRequest(evalCtx.Request, "ProblemId", "Id")
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
		"UploadAvatar":          {RequireAuth, RequireSelfOrAdmin("Username", "Id")},
		"DeleteAvatar":          {RequireAuth, RequireSelfOrAdmin("Username", "Id")},
		"UpdateUser":            {RequireAuth, RequireSelfOrAdmin("Username", "Id")},
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
	if targetUsername, err := extractStringFromRequest(evalCtx.Request, "Username", "username", "Id", "id"); err == nil && targetUsername != "" {
		cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
		if cleanTarget == strings.ToLower(evalCtx.User.Username) {
			return nil
		}
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}
	targetUserID, err := extractUUIDFromRequest(evalCtx.Request, "Id", "id")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user identifier is required for authorization")
	}
	if targetUserID != evalCtx.User.Id {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	return nil
}

func checkListUserSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	targetUsername, err := extractStringFromRequest(evalCtx.Request, "Username", "username")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid submissions request")
	}

	cleanTarget := strings.ToLower(strings.TrimPrefix(targetUsername, "@"))
	isSelf := cleanTarget == strings.ToLower(evalCtx.User.Username)

	if !isSelf && !evalCtx.User.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
	}

	if isSelf {
		if contestID, err := extractUUIDFromRequest(evalCtx.Request, "ContestId", "contest_id"); err == nil && contestID != uuid.Nil {
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
	var contestID uuid.UUID
	if contestLogin, err := extractStringFromRequest(evalCtx.Request, "ContestLogin", "contest_login"); err == nil && contestLogin != "" {
		orgLogin, err := extractStringFromRequest(evalCtx.Request, "OrgLogin", "OrganizationLogin", "org_login")
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
		contestID = contest.ID
	} else {
		var err error
		contestID, err = extractUUIDFromRequest(evalCtx.Request, "ContestId", "contest_id")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
		}
	}

	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contestID, evalCtx.User.Id, accessmodel.ActionGetMonitor)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	allowed, err = evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contestID, evalCtx.User.Id, accessmodel.ActionListOwnSubmissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
}

func checkListContestSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	orgLogin, err := extractStringFromRequest(evalCtx.Request, "OrgLogin", "org_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required")
	}
	contestLogin, err := extractStringFromRequest(evalCtx.Request, "ContestLogin", "contest_login")
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

	if targetUserID, err := extractUUIDFromRequest(evalCtx.Request, "UserId", "user_id"); err == nil && targetUserID != uuid.Nil {
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
	orgLogin, err := extractStringFromRequest(evalCtx.Request, "OrgLogin", "org_login")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "organization login is required")
	}
	contestLogin, err := extractStringFromRequest(evalCtx.Request, "ContestLogin", "contest_login")
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

	submissionID, err := extractUUIDFromRequest(evalCtx.Request, "SubmissionId", "submission_id", "id")
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

	targetUserID, err := extractUUIDFromRequest(evalCtx.Request, "UserId", "user_id")
	if err == nil && targetUserID == evalCtx.User.Id {
		return nil
	}

	return RequireContestPermission(accessmodel.ActionManageContest)(ctx, evalCtx)
}

func toUUID(val any) (uuid.UUID, error) {
	if val == nil {
		return uuid.Nil, fmt.Errorf("nil value")
	}
	switch v := val.(type) {
	case uuid.UUID:
		return v, nil
	case *uuid.UUID:
		if v == nil {
			return uuid.Nil, fmt.Errorf("nil uuid pointer")
		}
		return *v, nil
	case corev1.OptUUID:
		if v.IsSet() {
			return v.Value, nil
		}
		return uuid.Nil, fmt.Errorf("opt uuid not set")
	case string:
		if v == "" {
			return uuid.Nil, fmt.Errorf("empty string")
		}
		return uuid.Parse(v)
	case *string:
		if v == nil || *v == "" {
			return uuid.Nil, fmt.Errorf("empty string pointer")
		}
		return uuid.Parse(*v)
	case corev1.OptString:
		if v.IsSet() && v.Value != "" {
			return uuid.Parse(v.Value)
		}
		return uuid.Nil, fmt.Errorf("opt string not set")
	default:
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Pointer && !rv.IsNil() {
			return toUUID(rv.Elem().Interface())
		}
		return uuid.Nil, fmt.Errorf("unsupported uuid type %T", val)
	}
}

func toString(val any) (string, bool) {
	if val == nil {
		return "", false
	}
	switch v := val.(type) {
	case string:
		return v, true
	case *string:
		if v == nil {
			return "", false
		}
		return *v, true
	case corev1.OptString:
		if v.IsSet() {
			return v.Value, true
		}
		return "", false
	case corev1.OptNilString:
		if v.IsSet() {
			return v.Value, true
		}
		return "", false
	case uuid.UUID:
		return v.String(), true
	case *uuid.UUID:
		if v == nil {
			return "", false
		}
		return v.String(), true
	default:
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Pointer && !rv.IsNil() {
			return toString(rv.Elem().Interface())
		}
		return "", false
	}
}

func normalizeParamName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

func extractUUIDFromRequest(request interface{}, fieldNames ...string) (uuid.UUID, error) {
	if mreq, ok := request.(ogenMiddleware.Request); ok {
		for key, val := range mreq.Params {
			normKey := normalizeParamName(key.Name)
			for _, name := range fieldNames {
				if normKey == normalizeParamName(name) {
					if id, err := toUUID(val); err == nil && id != uuid.Nil {
						return id, nil
					}
				}
			}
		}
		if mreq.Body != nil {
			if id, err := extractUUIDFromRequest(mreq.Body, fieldNames...); err == nil && id != uuid.Nil {
				return id, nil
			}
		}
	}

	v := reflect.ValueOf(request)
	if !v.IsValid() {
		return uuid.Nil, fmt.Errorf("request is nil")
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return uuid.Nil, fmt.Errorf("request is nil")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return uuid.Nil, fmt.Errorf("request must be a struct")
	}

	id, found, err := extractUUIDFromStruct(v, fieldNames)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		return id, nil
	}

	params := v.FieldByName("Params")
	if params.IsValid() {
		id, found, err := extractUUIDFromValue(params, fieldNames)
		if err != nil {
			return uuid.Nil, err
		}
		if found {
			return id, nil
		}
	}

	return uuid.Nil, fmt.Errorf("uuid field not found")
}

func extractUUIDFromStruct(v reflect.Value, fieldNames []string) (uuid.UUID, bool, error) {
	for _, fieldName := range fieldNames {
		normFieldName := normalizeParamName(fieldName)
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if normalizeParamName(f.Name) == normFieldName {
				val := v.Field(i)
				if id, err := toUUID(val.Interface()); err == nil && id != uuid.Nil {
					return id, true, nil
				}
			}
		}
	}

	return uuid.Nil, false, nil
}

func extractUUIDFromValue(v reflect.Value, fieldNames []string) (uuid.UUID, bool, error) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return uuid.Nil, false, nil
		}
		v = v.Elem()
	}

	if !v.IsValid() {
		return uuid.Nil, false, nil
	}

	if v.Kind() == reflect.Struct {
		return extractUUIDFromStruct(v, fieldNames)
	}

	if id, err := toUUID(v.Interface()); err == nil && id != uuid.Nil {
		return id, true, nil
	}

	return uuid.Nil, false, nil
}

func extractStringFromRequest(request interface{}, fieldNames ...string) (string, error) {
	if mreq, ok := request.(ogenMiddleware.Request); ok {
		for key, val := range mreq.Params {
			normKey := normalizeParamName(key.Name)
			for _, name := range fieldNames {
				if normKey == normalizeParamName(name) {
					if s, ok := toString(val); ok && s != "" {
						return s, nil
					}
				}
			}
		}
		if mreq.Body != nil {
			if s, err := extractStringFromRequest(mreq.Body, fieldNames...); err == nil && s != "" {
				return s, nil
			}
		}
	}

	v := reflect.ValueOf(request)
	if !v.IsValid() {
		return "", fmt.Errorf("request is nil")
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", fmt.Errorf("request is nil")
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return "", fmt.Errorf("request must be a struct")
	}

	for _, fieldName := range fieldNames {
		normFieldName := normalizeParamName(fieldName)
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if normalizeParamName(f.Name) == normFieldName {
				val := v.Field(i)
				if s, ok := toString(val.Interface()); ok && s != "" {
					return s, nil
				}
			}
		}
	}

	params := v.FieldByName("Params")
	if params.IsValid() {
		p := params
		for p.Kind() == reflect.Pointer {
			if p.IsNil() {
				break
			}
			p = p.Elem()
		}
		if p.Kind() == reflect.Struct {
			for _, fieldName := range fieldNames {
				normFieldName := normalizeParamName(fieldName)
				for i := 0; i < p.NumField(); i++ {
					f := p.Type().Field(i)
					if normalizeParamName(f.Name) == normFieldName {
						val := p.Field(i)
						if s, ok := toString(val.Interface()); ok && s != "" {
							return s, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("string field not found")
}
