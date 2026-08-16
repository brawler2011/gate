package middleware

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type EvalContext struct {
	User        models.User
	Request     interface{}
	OperationID string
	Deps        strictAuthzDependencies
}

type AccessEvaluator func(ctx context.Context, evalCtx *EvalContext) error

type strictAuthzDependencies struct {
	permissionsUC interfaces.PermissionsUC
	submissionsUC interfaces.SubmissionsUC
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
		targetUserID, err := extractUUIDFromRequest(evalCtx.Request, idFieldNames...)
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "user id is required for authorization")
		}
		if targetUserID != evalCtx.User.Id && !evalCtx.User.IsAdmin() {
			return pkg.Wrap(pkg.NoPermission, nil, "can only modify your own resource")
		}
		return nil
	}
}

func RequireOrgPermission(action models.OrgAction) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() {
			return pkg.ErrUnauthenticated
		}
		orgID, err := extractUUIDFromRequest(evalCtx.Request, "OrganizationId", "OrgId", "Id")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "organization id is required for authorization")
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

func RequireContestPermission(action models.ContestAction) AccessEvaluator {
	return func(ctx context.Context, evalCtx *EvalContext) error {
		if evalCtx.User.IsGuest() && action != models.ActionGetContest && action != models.ActionGetContestProblem {
			return pkg.ErrUnauthenticated
		}
		contestID, err := extractUUIDFromRequest(evalCtx.Request, "ContestId", "Id")
		if err != nil {
			return pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
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

func RequireProblemPermission(action models.ProblemAction) AccessEvaluator {
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

		"GetMe":            {RequireAuth},
		"GetMyDashboard":   {RequireAuth},
		"CreateContest":    {RequireAuth},
		"GetMyContestRole": {RequireAuth},

		"ListOrganizations":        {RequireAuth},
		"CreateOrganization":       {RequireAuth},
		"GetOrganization":          {RequireAuth, RequireOrgPermission(models.ActionViewOrganization)},
		"UpdateOrganization":       {RequireAuth, RequireOrgPermission(models.ActionManageOrganization)},
		"DeleteOrganization":       {RequireAuth, RequireOrgPermission(models.ActionDeleteOrganization)},
		"ListOrganizationMembers":  {RequireAuth, RequireOrgPermission(models.ActionViewOrganization)},
		"AddOrganizationMember":    {RequireAuth, RequireOrgPermission(models.ActionManageOrganization)},
		"RemoveOrganizationMember": {RequireAuth, RequireOrgPermission(models.ActionManageOrganization)},

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

		"CreateProblem":        {RequireAuth},
		"ListUserContests":     {RequireAuth, checkListUserContestsAccess},
		"ListUserSubmissions":  {RequireAuth, checkListUserSubmissionsAccess},
		"ListWorkshopContests": {RequireAuth},
		"GetSubmission":        {RequireAuth, checkGetSubmissionAccess},
		"UploadAvatar":         {RequireAuth, RequireSelfOrAdmin("Id")},
		"DeleteAvatar":         {RequireAuth, RequireSelfOrAdmin("Id")},
		"UpdateUser":           {RequireAuth, RequireSelfOrAdmin("Id")},

		"ListAdminContests": {RequireAuth, RequireAdmin},
		"CreatePost":        {RequireAuth, RequireAdmin},
		"PatchPostById":     {RequireAuth, RequireAdmin},
		"DeletePostById":    {RequireAuth, RequireAdmin},
		"ListSubmissions":   {RequireAuth, RequireAdmin},

		"GetContest":             {RequireContestPermission(models.ActionGetContest)},
		"GetContestScoreboard":   {RequireContestPermission(models.ActionGetMonitor)},
		"UpdateContest":          {RequireAuth, RequireContestPermission(models.ActionUpdateContest)},
		"DeleteContest":          {RequireAuth, RequireContestPermission(models.ActionAdminContest)},
		"CreateContestProblem":   {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"GetContestProblem":      {RequireContestPermission(models.ActionGetContestProblem)},
		"DeleteContestProblem":   {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"CreateContestMember":    {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"UpdateContestMember":    {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"DeleteContestMember":    {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"ListContestMembers":     {RequireAuth, checkListContestMembersAccess},
		"ListContestTeams":       {RequireAuth, checkListContestMembersAccess},
		"CreateContestTeam":      {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"UpdateContestTeam":      {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"DeleteContestTeam":      {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"ListContestSubmissions": {RequireAuth, checkListContestSubmissionsAccess},
		"CreateSubmission":       {RequireAuth, RequireContestPermission(models.ActionCreateSubmission)},
		"RejudgeSubmission":     {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"RejudgeContestProblem": {RequireAuth, RequireContestPermission(models.ActionManageContest)},
		"RejudgeContest":        {RequireAuth, RequireContestPermission(models.ActionManageContest)},

		"GetProblem":           {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"GetProblemLimits":     {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"GetProblemStatement":  {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"UpdateProblem":        {RequireAuth, RequireProblemPermission(models.ActionEditProblem)},
		"DeleteProblem":        {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"ImportProblem":        {RequireAuth, RequireProblemPermission(models.ActionEditProblem)},
		"PublishProblem":       {RequireAuth, RequireProblemPermission(models.ActionEditProblem)},
		"ListProblemPackages":  {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"ListProblemMembers":   {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"CreateProblemMember":  {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"UpdateProblemMember":  {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"DeleteProblemMember":  {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"ListProblemTeams":     {RequireAuth, RequireProblemPermission(models.ActionViewProblem)},
		"CreateProblemTeam":    {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"UpdateProblemTeam":    {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
		"DeleteProblemTeam":    {RequireAuth, RequireProblemPermission(models.ActionAdminProblem)},
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
		policies[operationID] = []AccessEvaluator{RequireAuth, RequireProblemPermission(models.ActionEditProblem)}
	}

	return policies
}

// AuthzStrictMiddleware validates operation access before strict handlers are called.
func AuthzStrictMiddleware(permissionsUC interfaces.PermissionsUC, submissionsUC interfaces.SubmissionsUC) corev1.StrictMiddlewareFunc {
	deps := strictAuthzDependencies{
		permissionsUC: permissionsUC,
		submissionsUC: submissionsUC,
	}

	return func(next corev1.StrictHandlerFunc, operationID string) corev1.StrictHandlerFunc {
		evaluators, ok := endpointPolicies[operationID]
		if !ok || len(evaluators) == 0 {
			// Zero-Trust Default Deny
			return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
				return nil, pkg.Wrap(pkg.NoPermission, nil, "endpoint not permitted by authorization policy")
			}
		}

		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
			user := GetUser(ctx)
			evalCtx := &EvalContext{
				User:        user,
				Request:     request,
				OperationID: operationID,
				Deps:        deps,
			}

			for _, evaluator := range evaluators {
				if err := evaluator(ctx, evalCtx); err != nil {
					return nil, err
				}
			}

			return next(ctx, w, r, request)
		}
	}
}

func checkListUserContestsAccess(ctx context.Context, evalCtx *EvalContext) error {
	targetUserID, err := extractUUIDFromRequest(evalCtx.Request, "Id")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user id is required for authorization")
	}

	if targetUserID != evalCtx.User.Id && !evalCtx.User.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	return nil
}

func checkListUserSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	req, err := asListUserSubmissionsRequestObject(evalCtx.Request)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid submissions request")
	}

	if req.UserId != evalCtx.User.Id && !evalCtx.User.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
	}

	if req.UserId == evalCtx.User.Id && req.Params.ContestId != nil {
		allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(
			ctx,
			*req.Params.ContestId,
			evalCtx.User.Id,
			models.ActionListOwnSubmissions,
		)
		if err != nil {
			return err
		}
		if !allowed {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
	}

	return nil
}

func checkListContestMembersAccess(ctx context.Context, evalCtx *EvalContext) error {
	contestID, err := extractUUIDFromRequest(evalCtx.Request, "ContestId")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
	}

	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contestID, evalCtx.User.Id, models.ActionGetMonitor)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	allowed, err = evalCtx.Deps.permissionsUC.HasContestPermission(ctx, contestID, evalCtx.User.Id, models.ActionListOwnSubmissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
}

func checkListContestSubmissionsAccess(ctx context.Context, evalCtx *EvalContext) error {
	req, err := asListContestSubmissionsRequestObject(evalCtx.Request)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid contest submissions request")
	}

	action := models.ActionListUsersSubmissions
	errMessage := "insufficient permission to list all contest submissions"

	if req.Params.UserId != nil {
		if *req.Params.UserId == evalCtx.User.Id {
			action = models.ActionListOwnSubmissions
			errMessage = "insufficient permission to view own submissions in this contest"
		} else {
			action = models.ActionListUsersSubmissions
			errMessage = "insufficient permission to view other users' submissions"
		}
	}

	allowed, err := evalCtx.Deps.permissionsUC.HasContestPermission(ctx, req.ContestId, evalCtx.User.Id, action)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.Wrap(pkg.NoPermission, nil, errMessage)
	}

	return nil
}

func checkGetSubmissionAccess(ctx context.Context, evalCtx *EvalContext) error {
	if evalCtx.Deps.submissionsUC == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "submissions authorization dependency is not configured")
	}

	submissionID, err := extractUUIDFromRequest(evalCtx.Request, "SubmissionId")
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
			models.ActionGetOtherUserSubmission,
		)
		if err == nil && allowed {
			return nil
		}
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
}

func asListContestSubmissionsRequestObject(request interface{}) (corev1.ListContestSubmissionsRequestObject, error) {
	switch req := request.(type) {
	case corev1.ListContestSubmissionsRequestObject:
		return req, nil
	case *corev1.ListContestSubmissionsRequestObject:
		return *req, nil
	default:
		return corev1.ListContestSubmissionsRequestObject{}, fmt.Errorf("unexpected request type %T", request)
	}
}

func asListUserSubmissionsRequestObject(request interface{}) (corev1.ListUserSubmissionsRequestObject, error) {
	switch req := request.(type) {
	case corev1.ListUserSubmissionsRequestObject:
		return req, nil
	case *corev1.ListUserSubmissionsRequestObject:
		return *req, nil
	default:
		return corev1.ListUserSubmissionsRequestObject{}, fmt.Errorf("unexpected request type %T", request)
	}
}

var uuidType = reflect.TypeOf(uuid.UUID{})

func extractUUIDFromRequest(request interface{}, fieldNames ...string) (uuid.UUID, error) {
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
		field := v.FieldByName(fieldName)
		if !field.IsValid() {
			continue
		}

		id, found, err := extractUUIDFromValue(field, fieldNames)
		if err != nil {
			return uuid.Nil, false, err
		}
		if found {
			return id, true, nil
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

	if v.Type() == uuidType || v.Type().ConvertibleTo(uuidType) {
		id := v.Convert(uuidType).Interface().(uuid.UUID)
		return id, true, nil
	}

	if v.Kind() == reflect.Struct {
		return extractUUIDFromStruct(v, fieldNames)
	}

	if v.Kind() == reflect.String {
		if v.Len() == 0 {
			return uuid.Nil, false, nil
		}

		id, err := uuid.Parse(v.String())
		if err != nil {
			return uuid.Nil, false, err
		}
		return id, true, nil
	}

	return uuid.Nil, false, nil
}
