package permissions

import (
	"context"
	"embed"
	"fmt"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/types"
)

//go:embed opa/*.rego
var opaFS embed.FS

type ContestsUC interface {
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMemberRecord, error)
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type ProblemsUC interface {
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error)
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
}

type PermissionsUseCase struct {
	contestsUC   ContestsUC
	usersUC      UsersUC
	problemsUC   ProblemsUC
	contestQuery rego.PreparedEvalQuery
	problemQuery rego.PreparedEvalQuery
}

func NewUseCase(contestsUC ContestsUC, usersUC UsersUC, problemsUC ProblemsUC) *PermissionsUseCase {
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
		contestQuery: mustPrepare("data.authz.contest.permissions", "common.rego", "contest.rego"),
		problemQuery: mustPrepare("data.authz.problem.permissions", "common.rego", "problem.rego"),
	}
}

type ContestAction string

const (
	ActionGetContest             ContestAction = "GetContest"
	ActionUpdateContest          ContestAction = "UpdateContest"
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
	contest       *models.Contest
	contestMember *models.ContestMemberRecord
	problem       *models.Problem
	problemMember *models.ProblemMember
	user          *models.User
}

type PermissionOption func(*permissionOptions)

func WithUser(u *models.User) PermissionOption {
	return func(o *permissionOptions) {
		o.user = u
	}
}

func WithContest(c *models.Contest) PermissionOption {
	return func(o *permissionOptions) {
		o.contest = c
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
	perms, err := uc.getContestPermissions(ctx, contestID, userID, opts...)
	if err != nil {
		return false, err
	}

	switch action {
	case ActionGetContest:
		return perms.GetContest, nil
	case ActionUpdateContest:
		return perms.UpdateContest, nil
	case ActionAdminContest:
		return perms.AdminContest, nil
	case ActionGetMonitor:
		return perms.GetMonitor, nil
	case ActionListUsersSubmissions:
		return perms.ListUsersSubmissions, nil
	case ActionListOwnSubmissions:
		return perms.ListOwnSubmissions, nil
	case ActionGetOtherUserSubmission:
		return perms.GetOtherUserSubmission, nil
	case ActionGetOwnSubmission:
		return perms.GetOwnSubmission, nil
	case ActionCreateSubmission:
		return perms.CreateSubmission, nil
	default:
		return false, fmt.Errorf("unknown contest action: %s", action)
	}
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
	perms, err := uc.getProblemPermissions(ctx, problemID, userID, opts...)
	if err != nil {
		return false, err
	}

	switch action {
	case ActionGetProblem:
		return perms.ViewProblem, nil
	case ActionUpdateProblem:
		return perms.EditProblem, nil
	case ActionAdminProblem:
		return perms.AdminProblem, nil
	default:
		return false, fmt.Errorf("unknown problem action: %s", action)
	}
}
