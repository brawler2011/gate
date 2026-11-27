package postgres

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProblemsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewProblemsRepo(db *pgxpool.Pool) *ProblemsRepo {
	return &ProblemsRepo{
		pool:    db,
		queries: sqlc.New(db),
	}
}

func (r *ProblemsRepo) CreateProblem(ctx context.Context, params *problemsmodel.CreateProblemParams) error {
	_, err := r.queries.CreateProblem(ctx, sqlc.CreateProblemParams{
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		Visibility:     params.Visibility,
	})
	if err != nil {
		return dbError("create problem", err)
	}
	return dbError("insert problem draft", r.queries.InsertProblemDraft(ctx, sqlc.InsertProblemDraftParams{ProblemID: params.ID, Title: params.Title, ShortName: params.ShortName}))
}

func (r *ProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error) {
	problem, err := r.queries.GetProblemByID(ctx, id)
	if err != nil {
		return problemsmodel.Problem{}, dbError("get problem by id", err)
	}
	return mapGetProblemByIDRow(problem), nil
}

func (r *ProblemsRepo) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		return dbError("delete problem", err)
	}
	return nil
}

func (r *ProblemsRepo) ListProblems(ctx context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	descending := true
	if filter.Descending != nil {
		descending = *filter.Descending
	}
	viewer, all, err := (Access{q: r.queries}).visibility(ctx)
	if err != nil {
		return nil, err
	}
	items, total, err := scopeProblems(ctx, r.queries, sqlc.ScopeProblemsParams{
		ViewerID: viewer, All: all, OrganizationID: filter.OrganizationID,
		OwnerID: filter.OwnerID, Search: filter.Search, Visibility: filter.Visibility,
		IsTemplate: filter.IsTemplate, Descending: descending, PageSize: filter.PageSize,
		PageOffset: (int64(filter.Page) - 1) * int64(filter.PageSize),
	})
	if err != nil {
		return nil, err
	}
	return &problemsmodel.ProblemsList{Problems: items, Pagination: paging.NewPagination(filter.Page, filter.PageSize, total)}, nil
}

func (r *ProblemsRepo) UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error {
	err := r.queries.UpdateProblem(ctx, sqlc.UpdateProblemParams{
		ID:         id,
		Visibility: problem.Visibility,
		IsTemplate: problem.IsTemplate,
	})
	if err != nil {
		return dbError("update problem", err)
	}
	return nil
}

func (r *ProblemsRepo) CreateProblemMember(ctx context.Context, params *problemsmodel.CreateProblemMemberParams) error {
	err := r.queries.AddProblemMember(ctx, sqlc.AddProblemMemberParams{
		ProblemID: params.ProblemID,
		UserID:    params.UserID,
		Role:      params.Role,
	})
	if err != nil {
		return dbError("create problem member", err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error) {
	row, err := r.queries.GetProblemMember(ctx, sqlc.GetProblemMemberParams{
		ProblemID: problemId,
		UserID:    userId,
	})
	if err != nil {
		return problemsmodel.ProblemMember{}, dbError("get problem member", err)
	}
	return mapProblemMember(row), nil
}

func (r *ProblemsRepo) ListProblemMembers(ctx context.Context, problemID uuid.UUID, request paging.Request) ([]problemsmodel.ProblemMember, int32, error) {
	request, err := request.Normalize()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.queries.ListProblemMembers(ctx, sqlc.ListProblemMembersParams{ProblemID: problemID, PageSize: request.PageSize, PageOffset: request.Offset()})
	if err != nil {
		return nil, 0, dbError("list problem members", err)
	}

	members := make([]problemsmodel.ProblemMember, len(rows))
	for i, row := range rows {
		members[i] = mapListProblemMembersRow(row)
	}

	count, err := r.queries.CountProblemMembers(ctx, problemID)
	if err != nil {
		return nil, 0, dbError("list problem members", err)
	}
	return members, safeInt32(count), nil
}

func (r *ProblemsRepo) UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role accessmodel.ProblemRole) error {
	err := r.queries.UpdateProblemMemberRole(ctx, sqlc.UpdateProblemMemberRoleParams{
		ProblemID: problemID,
		UserID:    userID,
		Role:      role,
	})
	if err != nil {
		return dbError("update problem member role", err)
	}
	return nil
}

func (r *ProblemsRepo) RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error {
	err := r.queries.RemoveProblemMember(ctx, sqlc.RemoveProblemMemberParams{
		ProblemID: problemID,
		UserID:    userID,
	})
	if err != nil {
		return dbError("remove problem member", err)
	}
	return nil
}

func mapGetProblemByIDRow(p sqlc.GetProblemByIDRow) problemsmodel.Problem {
	return problemsmodel.Problem{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrgLogin,
		Visibility:        string(p.Visibility),
		Title:             p.Title,
		ShortName:         p.ShortName,
		TimeLimitMs:       widen(p.TimeLimitMs),
		MemoryLimitMb:     int(p.MemoryLimitMb),
		IsTemplate:        p.IsTemplate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

func mapProblemMember(pm sqlc.ProblemMember) problemsmodel.ProblemMember {
	return problemsmodel.ProblemMember{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Role:      accessmodel.ProblemRole(pm.Role),
		Username:  "",
		Email:     "",
		CreatedAt: pm.CreatedAt,
	}
}

func mapListProblemMembersRow(pm sqlc.ListProblemMembersRow) problemsmodel.ProblemMember {
	var email string
	if pm.Email != nil {
		email = *pm.Email
	}
	return problemsmodel.ProblemMember{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Role:      accessmodel.ProblemRole(pm.Role),
		Username:  pm.Username,
		Email:     email,
		CreatedAt: pm.CreatedAt,
	}
}
