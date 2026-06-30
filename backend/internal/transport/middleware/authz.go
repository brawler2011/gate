package middleware

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

type strictAuthzPolicy struct {
	public        bool
	requireAuth   bool
	requireAdmin  bool
	contestAction *models.ContestAction
	problemAction *models.ProblemAction
	customCheck   strictAuthzCustomCheck
}

type strictAuthzCustomCheck func(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error

type strictAuthzDependencies struct {
	permissionsUC interfaces.PermissionsUC
	submissionsUC interfaces.SubmissionsUC
}

var strictAuthzPolicies = buildStrictAuthzPolicies()

func buildStrictAuthzPolicies() map[string]strictAuthzPolicy {
	policies := map[string]strictAuthzPolicy{
		"GetHealth":           {public: true},
		"ListPublicContests":  {public: true},
		"ListPosts":           {public: true},
		"GetPostById":         {public: true},
		"GetPostImage":        {public: true},
		"GetPublishedPackage": {public: true},
		"ListProblems":        {public: true},
		"ListUsers":           {public: true},
		"GetUser":             {public: true},
		"GetUserAvatar":       {public: true},
		"Register":            {public: true},
		"Login":               {public: true},
		"Logout":              {public: true},

		"GetMe":            {requireAuth: true},
		"CreateContest":    {requireAuth: true},
		"GetMyContestRole": {requireAuth: true},

		"ListOrganizations":        {requireAuth: true},
		"CreateOrganization":       {requireAuth: true},
		"GetOrganization":          {requireAuth: true},
		"UpdateOrganization":       {requireAuth: true},
		"DeleteOrganization":       {requireAuth: true},
		"ListOrganizationMembers":  {requireAuth: true},
		"AddOrganizationMember":    {requireAuth: true},
		"RemoveOrganizationMember": {requireAuth: true},

		"ListTeams":        {requireAuth: true},
		"CreateTeam":       {requireAuth: true},
		"GetTeam":          {requireAuth: true},
		"UpdateTeam":       {requireAuth: true},
		"DeleteTeam":       {requireAuth: true},
		"ListTeamMembers":  {requireAuth: true},
		"AddTeamMember":    {requireAuth: true},
		"RemoveTeamMember": {requireAuth: true},

		"CreateProblem":        {requireAuth: true},
		"ListUserContests":     {requireAuth: true, customCheck: checkListUserContestsAccess},
		"ListUserSubmissions":  {requireAuth: true, customCheck: checkListUserSubmissionsAccess},
		"ListWorkshopContests": {requireAuth: true},
		"GetSubmission":        {requireAuth: true, customCheck: checkGetSubmissionAccess},
		"UploadAvatar":         {requireAuth: true, customCheck: checkAvatarSelfOrAdminAccess},
		"DeleteAvatar":         {requireAuth: true, customCheck: checkAvatarSelfOrAdminAccess},

		"ListAdminContests": {requireAdmin: true},
		"CreatePost":        {requireAdmin: true},
		"PatchPostById":     {requireAdmin: true},
		"DeletePostById":    {requireAdmin: true},
		"ListSubmissions":   {requireAdmin: true},

		"GetContest":             {contestAction: contestActionPtr(models.ActionGetContest)},
		"UpdateContest":          {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"DeleteContest":          {contestAction: contestActionPtr(models.ActionAdminContest)},
		"CreateContestProblem":   {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"GetContestProblem":      {contestAction: contestActionPtr(models.ActionGetContest)},
		"DeleteContestProblem":   {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"CreateContestMember":    {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"UpdateContestMember":    {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"DeleteContestMember":    {contestAction: contestActionPtr(models.ActionUpdateContest)},
		"ListContestMembers":     {requireAuth: true, customCheck: checkListContestMembersAccess},
		"ListContestSubmissions": {requireAuth: true, customCheck: checkListContestSubmissionsAccess},
		"CreateSubmission":       {contestAction: contestActionPtr(models.ActionCreateSubmission)},

		"GetProblem":          {problemAction: problemActionPtr(models.ActionViewProblem)},
		"UpdateProblem":       {problemAction: problemActionPtr(models.ActionEditProblem)},
		"DeleteProblem":       {problemAction: problemActionPtr(models.ActionAdminProblem)},
		"ImportProblem":       {problemAction: problemActionPtr(models.ActionEditProblem)},
		"PublishProblem":      {problemAction: problemActionPtr(models.ActionEditProblem)},
		"ListProblemPackages": {problemAction: problemActionPtr(models.ActionViewProblem)},
	}

	for _, operationID := range []string{
		"GetProblemLimits",
		"UpdateProblemLimits",
		"GetProblemStatement",
		"UpdateProblemStatement",
		"ListProblemCheckers",
		"CreateProblemChecker",
		"GetProblemChecker",
		"UpdateProblemChecker",
		"DeleteProblemChecker",
		"SetProblemCheckerMain",
		"ListProblemGenerators",
		"CreateProblemGenerator",
		"GetProblemGenerator",
		"UpdateProblemGenerator",
		"DeleteProblemGenerator",
		"SetProblemGeneratorMain",
		"ListProblemInteractors",
		"CreateProblemInteractor",
		"GetProblemInteractor",
		"UpdateProblemInteractor",
		"DeleteProblemInteractor",
		"SetProblemInteractorMain",
		"ListProblemValidators",
		"CreateProblemValidator",
		"GetProblemValidator",
		"UpdateProblemValidator",
		"DeleteProblemValidator",
		"SetProblemValidatorMain",
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
		policies[operationID] = strictAuthzPolicy{problemAction: problemActionPtr(models.ActionEditProblem)}
	}

	return policies
}

func contestActionPtr(action models.ContestAction) *models.ContestAction {
	a := action
	return &a
}

func problemActionPtr(action models.ProblemAction) *models.ProblemAction {
	a := action
	return &a
}

// AuthzStrictMiddleware validates operation access before strict handlers are called.
func AuthzStrictMiddleware(permissionsUC interfaces.PermissionsUC, submissionsUC interfaces.SubmissionsUC) corev1.StrictMiddlewareFunc {
	deps := strictAuthzDependencies{
		permissionsUC: permissionsUC,
		submissionsUC: submissionsUC,
	}

	return func(next corev1.StrictHandlerFunc, operationID string) corev1.StrictHandlerFunc {
		policy, ok := strictAuthzPolicies[operationID]
		if !ok {
			return next
		}

		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
			if policy.public {
				return next(ctx, w, r, request)
			}

			user := GetUser(ctx)
			if requiresAuthentication(policy) && user.IsGuest() {
				return nil, pkg.ErrUnauthenticated
			}

			if policy.requireAdmin && !user.IsAdmin() {
				return nil, pkg.Wrap(pkg.NoPermission, nil, "admin access required")
			}

			if policy.customCheck != nil {
				err := policy.customCheck(ctx, request, user, deps)
				if err != nil {
					return nil, err
				}
			}

			if policy.contestAction != nil {
				contestID, err := extractUUIDFromRequest(request, "ContestId")
				if err != nil {
					return nil, pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
				}

				allowed, err := deps.permissionsUC.HasContestPermission(ctx, contestID, user.Id, *policy.contestAction)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient contest permissions")
				}
			}

			if policy.problemAction != nil {
				problemID, err := extractUUIDFromRequest(request, "ProblemId", "Id")
				if err != nil {
					return nil, pkg.Wrap(pkg.ErrBadInput, err, "problem id is required for authorization")
				}

				allowed, err := deps.permissionsUC.HasProblemPermission(ctx, problemID, user.Id, *policy.problemAction)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient problem permissions")
				}
			}

			return next(ctx, w, r, request)
		}
	}
}

func checkAvatarSelfOrAdminAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	targetUserID, err := extractUUIDFromRequest(request, "Id")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user id is required for authorization")
	}

	if targetUserID != user.Id && !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "can only modify your own avatar")
	}

	return nil
}

func checkListUserContestsAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	targetUserID, err := extractUUIDFromRequest(request, "Id")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "user id is required for authorization")
	}

	if targetUserID != user.Id && !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	return nil
}

func checkListUserSubmissionsAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	req, err := asListUserSubmissionsRequestObject(request)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid submissions request")
	}

	if req.UserId != user.Id && !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can view other users' submissions")
	}

	if req.UserId == user.Id && req.Params.ContestId != nil {
		allowed, err := deps.permissionsUC.HasContestPermission(
			ctx,
			*req.Params.ContestId,
			user.Id,
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

func checkListContestMembersAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	contestID, err := extractUUIDFromRequest(request, "ContestId")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "contest id is required for authorization")
	}

	allowed, err := deps.permissionsUC.HasContestPermission(ctx, contestID, user.Id, models.ActionGetMonitor)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	allowed, err = deps.permissionsUC.HasContestPermission(ctx, contestID, user.Id, models.ActionListOwnSubmissions)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}

	return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
}

func checkListContestSubmissionsAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	req, err := asListContestSubmissionsRequestObject(request)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid contest submissions request")
	}

	action := models.ActionListUsersSubmissions
	errMessage := "insufficient permission to list all contest submissions"

	if req.Params.UserId != nil {
		if *req.Params.UserId == user.Id {
			action = models.ActionListOwnSubmissions
			errMessage = "insufficient permission to view own submissions in this contest"
		} else {
			action = models.ActionListUsersSubmissions
			errMessage = "insufficient permission to view other users' submissions"
		}
	}

	allowed, err := deps.permissionsUC.HasContestPermission(ctx, req.ContestId, user.Id, action)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.Wrap(pkg.NoPermission, nil, errMessage)
	}

	return nil
}

func checkGetSubmissionAccess(
	ctx context.Context,
	request interface{},
	user models.User,
	deps strictAuthzDependencies,
) error {
	if deps.submissionsUC == nil {
		return pkg.Wrap(pkg.ErrInternal, nil, "submissions authorization dependency is not configured")
	}

	submissionID, err := extractUUIDFromRequest(request, "SubmissionId")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "submission id is required for authorization")
	}

	submission, err := deps.submissionsUC.GetSubmission(ctx, submissionID)
	if err != nil {
		return err
	}

	if submission.CreatedBy == nil || *submission.CreatedBy != user.Id {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
	}

	return nil
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

func requiresAuthentication(policy strictAuthzPolicy) bool {
	return policy.requireAuth || policy.requireAdmin || policy.contestAction != nil || policy.problemAction != nil || policy.customCheck != nil
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
