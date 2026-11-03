package postgres

import (
	"context"

	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
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
		OwnerID:        params.OwnerID,
		Visibility:     params.Visibility,
		Title:          params.Title,
		ShortName:      params.ShortName,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemById(ctx context.Context, id uuid.UUID) (problemsmodel.Problem, error) {
	problem, err := r.queries.GetProblemByID(ctx, id)
	if err != nil {
		return problemsmodel.Problem{}, HandlePgErr(err)
	}
	return mapGetProblemByIDRow(problem), nil
}

func (r *ProblemsRepo) DeleteProblem(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteProblem(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) ListProblems(ctx context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	// Calculate offset
	offset := (filter.Page - 1) * filter.PageSize

	total, err := r.queries.CountAllProblems(ctx, sqlc.CountAllProblemsParams{
		OrganizationID: filter.OrganizationID,
		OwnerID:        filter.OwnerID,
		ViewerID:       filter.ViewerID,
		Search:         filter.Search,
		Visibility:     filter.Visibility,
		IsTemplate:     filter.IsTemplate,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	rows, err := r.queries.ListAllProblems(ctx, sqlc.ListAllProblemsParams{
		OrganizationID: filter.OrganizationID,
		OwnerID:        filter.OwnerID,
		ViewerID:       filter.ViewerID,
		Search:         filter.Search,
		Visibility:     filter.Visibility,
		IsTemplate:     filter.IsTemplate,
		PageLimit:      filter.PageSize,
		PageOffset:     offset,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	problems := make([]problemsmodel.Problem, len(rows))
	for i, row := range rows {
		problems[i] = mapListAllProblemsRow(row)
	}

	return &problemsmodel.ProblemsList{Problems: problems, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(total))}, nil
}

func (r *ProblemsRepo) UpdateProblem(ctx context.Context, id uuid.UUID, problem *problemsmodel.ProblemUpdate) error {
	err := r.queries.UpdateProblem(ctx, sqlc.UpdateProblemParams{
		ID:         id,
		Title:      problem.Title,
		Visibility: problem.Visibility,
		OwnerID:    problem.OwnerID,
		IsTemplate: problem.IsTemplate,
	})
	if err != nil {
		return HandlePgErr(err)
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
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (problemsmodel.ProblemMember, error) {
	row, err := r.queries.GetProblemMember(ctx, sqlc.GetProblemMemberParams{
		ProblemID: problemId,
		UserID:    userId,
	})
	if err != nil {
		return problemsmodel.ProblemMember{}, HandlePgErr(err)
	}
	return mapProblemMember(row), nil
}

func (r *ProblemsRepo) GetProblemTeams(ctx context.Context, problemId uuid.UUID) ([]problemsmodel.ProblemTeam, error) {
	rows, err := r.queries.ListProblemTeams(ctx, problemId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	teams := make([]problemsmodel.ProblemTeam, len(rows))
	for i, row := range rows {
		teams[i] = mapProblemTeam(row)
	}

	return teams, nil
}

func (r *ProblemsRepo) AddProblemTeam(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error {
	err := r.queries.AddProblemTeam(ctx, sqlc.AddProblemTeamParams{
		ProblemID:  problemID,
		TeamID:     teamID,
		Permission: permission,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) UpdateProblemTeamPermission(ctx context.Context, problemID, teamID uuid.UUID, permission problemsmodel.ProblemPermission) error {
	err := r.queries.UpdateProblemTeamPermission(ctx, sqlc.UpdateProblemTeamPermissionParams{
		ProblemID:  problemID,
		TeamID:     teamID,
		Permission: permission,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) RemoveProblemTeam(ctx context.Context, problemID, teamID uuid.UUID) error {
	err := r.queries.RemoveProblemTeam(ctx, sqlc.RemoveProblemTeamParams{
		ProblemID: problemID,
		TeamID:    teamID,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) ListProblemMembers(ctx context.Context, problemID uuid.UUID) ([]problemsmodel.ProblemMember, error) {
	rows, err := r.queries.ListProblemMembers(ctx, problemID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	members := make([]problemsmodel.ProblemMember, len(rows))
	for i, row := range rows {
		members[i] = mapListProblemMembersRow(row)
	}

	return members, nil
}

func (r *ProblemsRepo) UpdateProblemMemberRole(ctx context.Context, problemID, userID uuid.UUID, role problemsmodel.ProblemRole) error {
	err := r.queries.UpdateProblemMemberRole(ctx, sqlc.UpdateProblemMemberRoleParams{
		ProblemID: problemID,
		UserID:    userID,
		Role:      role,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ProblemsRepo) RemoveProblemMember(ctx context.Context, problemID, userID uuid.UUID) error {
	err := r.queries.RemoveProblemMember(ctx, sqlc.RemoveProblemMemberParams{
		ProblemID: problemID,
		UserID:    userID,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func mapGetProblemByIDRow(p sqlc.GetProblemByIDRow) problemsmodel.Problem {
	return problemsmodel.Problem{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrgLogin,
		OwnerID:           p.OwnerID,
		Visibility:        string(p.Visibility),
		Title:             p.Title,
		ShortName:         p.ShortName,
		TimeLimitMs:       int(p.TimeLimitMs),
		MemoryLimitMb:     int(p.MemoryLimitMb),
		IsTemplate:        p.IsTemplate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}
}

func (r *ProblemsRepo) GetWorkspaceHash(ctx context.Context, id uuid.UUID) (string, error) {
	hash, err := r.queries.GetWorkspaceHash(ctx, id)
	if err != nil {
		return "", HandlePgErr(err)
	}
	return hash, nil
}

func (r *ProblemsRepo) CommitWorkspace(ctx context.Context, id uuid.UUID, previousHash, hash string, metadata *problemsmodel.WorkspaceMetadata) error {
	params := sqlc.CommitWorkspaceParams{ID: id, PreviousHash: previousHash, Hash: hash}
	if metadata != nil {
		params.Title = &metadata.Title
		timeMs := safeInt32(metadata.TimeLimitMs)
		params.TimeMs = &timeMs
		memoryMb := safeInt32(metadata.MemoryLimitMb)
		params.MemoryMb = &memoryMb
	}
	rows, err := r.queries.CommitWorkspace(ctx, params)
	if err != nil {
		return HandlePgErr(err)
	}
	if rows == 0 {
		return pkg.Wrap(pkg.ErrConflict, nil, "workspace changed; reload before saving")
	}
	return nil
}

func mapProblemMember(pm sqlc.ProblemMember) problemsmodel.ProblemMember {
	return problemsmodel.ProblemMember{
		ProblemID: pm.ProblemID,
		UserID:    pm.UserID,
		Role:      problemsmodel.ProblemRole(pm.Role),
		Username:  "",
		Email:     "",
		CreatedAt: pm.CreatedAt,
	}
}

func mapProblemTeam(pt sqlc.ListProblemTeamsRow) problemsmodel.ProblemTeam {
	return problemsmodel.ProblemTeam{
		ProblemID:  pt.ProblemID,
		TeamID:     pt.TeamID,
		Permission: problemsmodel.ProblemPermission(pt.Permission),
		TeamName:   pt.TeamName,
		TeamSlug:   pt.TeamSlug,
		CreatedAt:  pt.CreatedAt,
	}
}

func (r *ProblemsRepo) ListDashboardProblems(ctx context.Context, userID uuid.UUID, limit int32) ([]problemsmodel.DashboardProblem, error) {
	rows, err := r.queries.ListDashboardProblems(ctx, sqlc.ListDashboardProblemsParams{
		OwnerID: &userID,
		Limit:   limit,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	problems := make([]problemsmodel.DashboardProblem, len(rows))
	for i, row := range rows {
		problems[i] = mapDashboardProblem(row)
	}

	return problems, nil
}

func mapListAllProblemsRow(p sqlc.ListAllProblemsRow) problemsmodel.Problem {
	return problemsmodel.Problem{
		ID:                p.ID,
		OrganizationID:    p.OrganizationID,
		OrganizationLogin: p.OrgLogin,
		OwnerID:           p.OwnerID,
		Visibility:        string(p.Visibility),
		Title:             p.Title,
		ShortName:         p.ShortName,
		TimeLimitMs:       int(p.TimeLimitMs),
		MemoryLimitMb:     int(p.MemoryLimitMb),
		IsTemplate:        p.IsTemplate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
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
		Role:      problemsmodel.ProblemRole(pm.Role),
		Username:  pm.Username,
		Email:     email,
		CreatedAt: pm.CreatedAt,
	}
}

func mapDashboardProblem(row sqlc.ListDashboardProblemsRow) problemsmodel.DashboardProblem {
	return problemsmodel.DashboardProblem{
		ID:                row.ProblemID,
		Title:             row.ProblemTitle,
		OrganizationID:    row.OrgID,
		OrganizationName:  row.OrgName,
		OrganizationLogin: row.OrgLogin,
		TimeLimitMs:       int(row.TimeLimitMs),
		MemoryLimitMb:     int(row.MemoryLimitMb),
		UpdatedAt:         row.UpdatedAt,
	}
}

func (r *ProblemsRepo) LockProblem(ctx context.Context, id uuid.UUID) error {
	_, err := r.queries.LockProblem(ctx, id)
	return HandlePgErr(err)
}
