package permissions

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/gate149/core/internal/cache"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/types"
)

//go:embed opa/*.rego
var opaFS embed.FS

type ContestsUC interface {
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (domain.ContestMember, error)
	GetContest(ctx context.Context, id uuid.UUID) (domain.Contest, error)
}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type ProblemsUC interface {
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (domain.ProblemMember, error)
	GetProblemById(ctx context.Context, id uuid.UUID) (domain.Problem, error)
}

type PermissionsUseCase struct {
	contestsUC   ContestsUC
	usersUC      UsersUC
	problemsUC   ProblemsUC
	cache        cache.Cache
	contestQuery rego.PreparedEvalQuery
	problemQuery rego.PreparedEvalQuery
}

func NewUseCase(contestsUC ContestsUC, usersUC UsersUC, problemsUC ProblemsUC, cache cache.Cache) *PermissionsUseCase {
	ctx := context.Background()

	// Define custom functions
	funcs := []func(*rego.Rego){
		rego.Function1(
			&rego.Function{
				Name:    "custom.get_contest",
				Decl:    types.NewFunction(types.Args(types.S), types.A),
				Memoize: true,
			},
			func(rctx rego.BuiltinContext, op1 *ast.Term) (*ast.Term, error) {
				idStr, ok := op1.Value.(ast.String)
				if !ok {
					return nil, fmt.Errorf("invalid argument type for contest_id")
				}
				id, err := uuid.Parse(string(idStr))
				if err != nil {
					return nil, err
				}
				contest, err := contestsUC.GetContest(rctx.Context, id)
				if err != nil {
					return nil, err
				}
				val, err := ast.InterfaceToValue(contest)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(val), nil
			},
		),
		rego.Function2(
			&rego.Function{
				Name:    "custom.get_contest_member",
				Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
				Memoize: true,
			},
			func(rctx rego.BuiltinContext, op1, op2 *ast.Term) (*ast.Term, error) {
				cIDStr, ok1 := op1.Value.(ast.String)
				uIDStr, ok2 := op2.Value.(ast.String)
				if !ok1 || !ok2 {
					return nil, fmt.Errorf("invalid argument types")
				}
				cID, err := uuid.Parse(string(cIDStr))
				if err != nil {
					return nil, err
				}
				uID, err := uuid.Parse(string(uIDStr))
				if err != nil {
					return nil, err
				}
				member, err := contestsUC.GetContestMember(rctx.Context, &models.ContestPermissionGet{
					ContestId: cID,
					UserId:    uID,
				})
				if err != nil {
					// Log debug info if needed, but error means not found usually
					// fmt.Printf("GetContestMember error: %v for cID=%s uID=%s\n", err, cID, uID)
					return nil, nil // Return nil if not found (no permission)
				}
				val, err := ast.InterfaceToValue(member)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(val), nil
			},
		),
		rego.Function1(
			&rego.Function{
				Name:    "custom.get_problem",
				Decl:    types.NewFunction(types.Args(types.S), types.A),
				Memoize: true,
			},
			func(rctx rego.BuiltinContext, op1 *ast.Term) (*ast.Term, error) {
				idStr, ok := op1.Value.(ast.String)
				if !ok {
					return nil, fmt.Errorf("invalid argument type for problem_id")
				}
				id, err := uuid.Parse(string(idStr))
				if err != nil {
					return nil, err
				}
				problem, err := problemsUC.GetProblemById(rctx.Context, id)
				if err != nil {
					return nil, err
				}
				val, err := ast.InterfaceToValue(problem)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(val), nil
			},
		),
		rego.Function2(
			&rego.Function{
				Name:    "custom.get_problem_member",
				Decl:    types.NewFunction(types.Args(types.S, types.S), types.A),
				Memoize: true,
			},
			func(rctx rego.BuiltinContext, op1, op2 *ast.Term) (*ast.Term, error) {
				pIDStr, ok1 := op1.Value.(ast.String)
				uIDStr, ok2 := op2.Value.(ast.String)
				if !ok1 || !ok2 {
					return nil, fmt.Errorf("invalid argument types")
				}
				pID, err := uuid.Parse(string(pIDStr))
				if err != nil {
					return nil, err
				}
				uID, err := uuid.Parse(string(uIDStr))
				if err != nil {
					return nil, err
				}
				member, err := problemsUC.GetProblemMember(rctx.Context, pID, uID)
				if err != nil {
					return nil, nil
				}
				val, err := ast.InterfaceToValue(member)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(val), nil
			},
		),
		rego.Function1(
			&rego.Function{
				Name:    "custom.get_user",
				Decl:    types.NewFunction(types.Args(types.S), types.A),
				Memoize: true,
			},
			func(rctx rego.BuiltinContext, op1 *ast.Term) (*ast.Term, error) {
				idStr, ok := op1.Value.(ast.String)
				if !ok {
					return nil, fmt.Errorf("invalid argument type for user_id")
				}
				id, err := uuid.Parse(string(idStr))
				if err != nil {
					return nil, err
				}
				user, err := usersUC.GetUserById(rctx.Context, id)
				if err != nil {
					return nil, err
				}
				val, err := ast.InterfaceToValue(user)
				if err != nil {
					return nil, err
				}
				return ast.NewTerm(val), nil
			},
		),
	}

	mustPrepare := func(query string, modules ...string) rego.PreparedEvalQuery {
		opts := []func(*rego.Rego){
			rego.Query(query),
		}
		opts = append(opts, funcs...)

		for _, m := range modules {
			content, err := opaFS.ReadFile("opa/" + m)
			if err != nil {
				panic(fmt.Errorf("failed to read rego module %s: %w", m, err))
			}
			opts = append(opts, rego.Module(m, string(content)))
		}

		r, err := rego.New(opts...).PrepareForEval(ctx)
		if err != nil {
			panic(fmt.Errorf("failed to prepare rego query %s: %w", query, err))
		}
		return r
	}

	return &PermissionsUseCase{
		contestsUC:   contestsUC,
		usersUC:      usersUC,
		problemsUC:   problemsUC,
		cache:        cache,
		contestQuery: mustPrepare("data.authz.contest.permissions", "common.rego", "contest.rego"),
		problemQuery: mustPrepare("data.authz.problem.permissions", "common.rego", "problem.rego"),
	}
}

type ContestAction string

const (
	ActionGetContest             ContestAction = "GetContest"
	ActionUpdateContest          ContestAction = "UpdateContest"
	ActionManageContest          ContestAction = "ManageContest"
	ActionAdminContest           ContestAction = "AdminContest"
	ActionGetMonitor             ContestAction = "GetMonitor"
	ActionListUsersSubmissions   ContestAction = "ListUsersSubmissions"
	ActionListOwnSubmissions     ContestAction = "ListOwnSubmissions"
	ActionGetOtherUserSubmission ContestAction = "GetOtherUserSubmission"
	ActionGetOwnSubmission       ContestAction = "GetOwnSubmission"
	ActionCreateSubmission       ContestAction = "CreateSubmission"
)

type ProblemAction string

const (
	ActionGetProblem    ProblemAction = "GetProblem"
	ActionUpdateProblem ProblemAction = "UpdateProblem"
	ActionAdminProblem  ProblemAction = "AdminProblem"
)

type permissionOptions struct {
	contest       *domain.Contest
	contestMember *domain.ContestMember
	problem       *domain.Problem
	problemMember *domain.ProblemMember
	user          *domain.User
}

type PermissionOption func(*permissionOptions)

func WithUser(u domain.User) PermissionOption {
	return func(o *permissionOptions) {
		o.user = &u
	}
}

func WithContest(c domain.Contest) PermissionOption {
	return func(o *permissionOptions) {
		o.contest = &c
	}
}

func (uc *PermissionsUseCase) getContestPermissions(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, opts ...PermissionOption) (*models.ContestPermissions, error) {
	options := &permissionOptions{}
	for _, opt := range opts {
		opt(options)
	}

	input := map[string]interface{}{
		"contest_id": contestID.String(),
		"user_id":    userID.String(),
	}

	if options.contest != nil {
		input["contest"] = options.contest
	}
	if options.contestMember != nil {
		input["member"] = options.contestMember
	}
	if options.user != nil {
		input["user"] = options.user
	}

	results, err := uc.contestQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, err
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return &models.ContestPermissions{}, nil
	}

	permsMap, ok := results[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from OPA: %T", results[0].Expressions[0].Value)
	}

	getBool := func(key string) bool {
		val, ok := permsMap[key]
		if !ok {
			return false
		}
		b, ok := val.(bool)
		return ok && b
	}

	return &models.ContestPermissions{
		GetContest:             getBool(string(ActionGetContest)),
		UpdateContest:          getBool(string(ActionUpdateContest)),
		ManageContest:          getBool(string(ActionManageContest)),
		AdminContest:           getBool(string(ActionAdminContest)),
		GetMonitor:             getBool(string(ActionGetMonitor)),
		ListUsersSubmissions:   getBool(string(ActionListUsersSubmissions)),
		ListOwnSubmissions:     getBool(string(ActionListOwnSubmissions)),
		GetOtherUserSubmission: getBool(string(ActionGetOtherUserSubmission)),
		GetOwnSubmission:       getBool(string(ActionGetOwnSubmission)),
		CreateSubmission:       getBool(string(ActionCreateSubmission)),
	}, nil
}

func (uc *PermissionsUseCase) HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action ContestAction, opts ...PermissionOption) (bool, error) {
	// Cache-aside for permission result
	// Note: options (user object etc) might make cache key complex.
	// We only cache based on IDs. If options are provided, we assume they match DB state or are consistent.
	// Actually, if we pass User object, it might be fresher than DB.
	// But OPA evaluation is deterministic given inputs.
	// Ideally we cache the result of (contestID, userID, action).
	// If inputs change (e.g. user role updated), cache should be invalidated.
	// We handle invalidation in other modules.

	key := cache.PermissionKey(userID, contestID, string(action))
	var allowed bool
	if err := uc.cache.Get(ctx, key, &allowed); err == nil {
		return allowed, nil
	}

	perms, err := uc.getContestPermissions(ctx, contestID, userID, opts...)
	if err != nil {
		return false, err
	}

	var result bool
	switch action {
	case ActionGetContest:
		result = perms.GetContest
	case ActionUpdateContest:
		result = perms.UpdateContest
	case ActionAdminContest:
		result = perms.AdminContest
	case ActionGetMonitor:
		result = perms.GetMonitor
	case ActionListUsersSubmissions:
		result = perms.ListUsersSubmissions
	case ActionListOwnSubmissions:
		result = perms.ListOwnSubmissions
	case ActionGetOtherUserSubmission:
		result = perms.GetOtherUserSubmission
	case ActionGetOwnSubmission:
		result = perms.GetOwnSubmission
	case ActionCreateSubmission:
		result = perms.CreateSubmission
	default:
		return false, fmt.Errorf("unknown contest action: %s", action)
	}

	// Cache result (short TTL)
	if err := uc.cache.Set(ctx, key, result, cache.PermissionTTL); err != nil {
		slog.Error("failed to cache contest permissions", "error", err, "key", key)
	}

	return result, nil
}

func (uc *PermissionsUseCase) getProblemPermissions(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, opts ...PermissionOption) (*models.ProblemPermissions, error) {
	options := &permissionOptions{}
	for _, opt := range opts {
		opt(options)
	}

	input := map[string]interface{}{
		"problem_id": problemID.String(),
		"user_id":    userID.String(),
	}

	if options.problem != nil {
		input["problem"] = options.problem
	}
	if options.problemMember != nil {
		input["member"] = options.problemMember
	}
	if options.user != nil {
		input["user"] = options.user
	}

	results, err := uc.problemQuery.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, err
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return &models.ProblemPermissions{}, nil
	}

	permsMap, ok := results[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from OPA")
	}

	getBool := func(key string) bool {
		val, ok := permsMap[key]
		if !ok {
			return false
		}
		b, ok := val.(bool)
		return ok && b
	}

	return &models.ProblemPermissions{
		ViewProblem:  getBool(string(ActionGetProblem)),
		EditProblem:  getBool(string(ActionUpdateProblem)),
		AdminProblem: getBool(string(ActionAdminProblem)),
	}, nil
}

func (uc *PermissionsUseCase) HasProblemPermission(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, action ProblemAction, opts ...PermissionOption) (bool, error) {
	key := cache.PermissionKey(userID, problemID, string(action))
	var allowed bool
	if err := uc.cache.Get(ctx, key, &allowed); err == nil {
		return allowed, nil
	}

	perms, err := uc.getProblemPermissions(ctx, problemID, userID, opts...)
	if err != nil {
		return false, err
	}

	var result bool
	switch action {
	case ActionGetProblem:
		result = perms.ViewProblem
	case ActionUpdateProblem:
		result = perms.EditProblem
	case ActionAdminProblem:
		result = perms.AdminProblem
	default:
		return false, fmt.Errorf("unknown problem action: %s", action)
	}

	if err := uc.cache.Set(ctx, key, result, cache.PermissionTTL); err != nil {
		slog.Error("failed to cache problem permissions", "error", err, "key", key)
	}

	return result, nil
}
