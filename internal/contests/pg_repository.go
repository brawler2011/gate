package contests

import (
	"context"

	contestssqlc "github.com/gate149/core/internal/contests/sqlc"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *contestssqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		queries: contestssqlc.New(db),
	}
}

func (r *Repository) CreateContest(ctx context.Context, params *models.CreateContestParams) error {
	_, err := r.queries.CreateContest(ctx, contestssqlc.CreateContestParams{
		ID:        params.Id,
		Title:     params.Title,
		CreatedBy: params.UserId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetContest(ctx context.Context, id uuid.UUID) (contestssqlc.Contest, error) {
	contest, err := r.queries.GetContest(ctx, id)
	if err != nil {
		return contestssqlc.Contest{}, pkg.HandlePgErr(err)
	}
	return contest, nil
}

func (r *Repository) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	err := r.queries.UpdateContest(ctx, contestssqlc.UpdateContestParams{
		ID:                     c.Id,
		Title:                  c.Title,
		Description:            c.Description,
		Visibility:             stringToNullContestVisibility(c.Visibility),
		MonitorScope:           stringToNullContestRole(c.MonitorScope),
		SubmissionsListScope:   stringToNullContestRole(c.SubmissionsListScope),
		SubmissionsReviewScope: stringToNullContestRole(c.SubmissionsReviewScope),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) DeleteContest(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteContest(ctx, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]contestssqlc.Contest, int64, error) {
	contests, err := r.queries.ListAdminContests(ctx, contestssqlc.ListAdminContestsParams{
		Limit:      int32(filter.PageSize),
		Offset:     int32(filter.Offset()),
		Search:     filter.Search,
		Visibility: filter.Visibility,
		SortBy:     filter.SortBy,
		SortOrder:  filter.SortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountAdminContests(ctx, contestssqlc.CountAdminContestsParams{
		Search:     filter.Search,
		Visibility: filter.Visibility,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return contests, count, nil
}

func (r *Repository) ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]contestssqlc.Contest, int64, error) {
	contests, err := r.queries.ListUserContests(ctx, contestssqlc.ListUserContestsParams{
		CreatedBy: filter.UserId,
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		Search:    filter.Search,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountUserContests(ctx, contestssqlc.CountUserContestsParams{
		UserID: filter.UserId,
		Search: filter.Search,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return contests, count, nil
}

func (r *Repository) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]contestssqlc.Contest, int64, error) {
	contests, err := r.queries.ListWorkshopContests(ctx, contestssqlc.ListWorkshopContestsParams{
		UserID:    filter.UserId,
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		Search:    filter.Search,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountWorkshopContests(ctx, contestssqlc.CountWorkshopContestsParams{
		UserID: filter.UserId,
		Search: filter.Search,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return contests, count, nil
}

func (r *Repository) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]contestssqlc.Contest, int64, error) {
	contests, err := r.queries.ListPublicContests(ctx, contestssqlc.ListPublicContestsParams{
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
		Search:    filter.Search,
		SortBy:    filter.SortBy,
		SortOrder: filter.SortOrder,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountPublicContests(ctx, filter.Search)
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return contests, count, nil
}

func (r *Repository) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	err := r.queries.CreateContestProblem(ctx, contestssqlc.CreateContestProblemParams{
		ProblemID: c.ProblemId,
		ContestID: c.ContestId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	err := r.queries.DeleteContestProblem(ctx, contestssqlc.DeleteContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (contestssqlc.GetContestProblemRow, error) {
	row, err := r.queries.GetContestProblem(ctx, contestssqlc.GetContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return contestssqlc.GetContestProblemRow{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestssqlc.GetContestProblemsRow, error) {
	rows, err := r.queries.GetContestProblems(ctx, contestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return rows, nil
}

func (r *Repository) ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]contestssqlc.ListContestMembersRow, int64, error) {
	rows, err := r.queries.ListContestMembers(ctx, contestssqlc.ListContestMembersParams{
		ContestID: filter.ContestId,
		Limit:     int32(filter.PageSize),
		Offset:    int32(filter.Offset()),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, count, nil
}

func (r *Repository) CreateContestMember(ctx context.Context, c *models.ContestMemberParams) error {
	err := r.queries.CreateContestMember(ctx, contestssqlc.CreateContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
		Role:      contestssqlc.ContestRole(c.Role),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (contestssqlc.GetContestMemberRow, error) {
	row, err := r.queries.GetContestMember(ctx, contestssqlc.GetContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
	})
	if err != nil {
		return contestssqlc.GetContestMemberRow{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error {
	err := r.queries.DeleteContestMember(ctx, contestssqlc.DeleteContestMemberParams{
		UserID:    userId,
		ContestID: contestId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error {
	err := r.queries.UpdateContestMember(ctx, contestssqlc.UpdateContestMemberParams{
		ContestID: contestId,
		UserID:    userId,
		Role:      contestssqlc.ContestRole(role),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// Helper functions to convert between sqlc types and models

func stringToNullContestVisibility(s *string) contestssqlc.NullContestVisibility {
	if s == nil {
		return contestssqlc.NullContestVisibility{Valid: false}
	}
	return contestssqlc.NullContestVisibility{
		ContestVisibility: contestssqlc.ContestVisibility(*s),
		Valid:             true,
	}
}

func stringToNullContestRole(s *string) contestssqlc.NullContestRole {
	if s == nil {
		return contestssqlc.NullContestRole{Valid: false}
	}
	return contestssqlc.NullContestRole{
		ContestRole: contestssqlc.ContestRole(*s),
		Valid:       true,
	}
}
