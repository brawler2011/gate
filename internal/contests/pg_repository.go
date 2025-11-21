package contests

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	_ "embed"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

//go:embed sql/create_contest.sql
var CreateContestQuery string

func (r *Repository) CreateContest(ctx context.Context, params *models.CreateContestParams) error {
	_, err := r.db.QueryxContext(ctx, CreateContestQuery,
		params.Id,
		params.Title,
		params.UserId,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

//go:embed sql/get_contest.sql
var GetContestQuery string

func (r *Repository) GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	var contest models.Contest
	err := r.db.GetContext(ctx, &contest, GetContestQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return &contest, nil
}

//go:embed sql/update_contest.sql
var UpdateContestQuery string

func (r *Repository) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	_, err := r.db.ExecContext(ctx, UpdateContestQuery,
		c.Id,
		c.Title,
		c.Description,
		c.Visibility,
		c.MonitorScope,
		c.SubmissionsListScope,
		c.SubmissionsReviewScope,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/delete_contest.sql
var DeleteContestQuery string

func (r *Repository) DeleteContest(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, DeleteContestQuery, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/list_contests.sql
var ListContestsQuery string

//go:embed sql/count_contests.sql
var CountContestsQuery string

func (r *Repository) ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error) {
	contests := make([]*models.Contest, 0)
	err := r.db.SelectContext(ctx, &contests, ListContestsQuery,
		filter.OwnerId,
		filter.Search,
		filter.Descending,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, CountContestsQuery,
		filter.OwnerId,
		filter.Search,
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &models.ContestsList{
		Contests: contests,
		Pagination: models.Pagination{
			Total: models.Total(count, filter.PageSize),
			Page:  filter.Page,
		},
	}, nil
}

//go:embed sql/create_contest_problem.sql
var CreateContestProblemQuery string

func (r *Repository) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	_, err := r.db.ExecContext(ctx, CreateContestProblemQuery, c.ProblemId, c.ContestId)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/delete_contest_problem.sql
var DeleteContestProblemQuery string

func (r *Repository) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	_, err := r.db.ExecContext(ctx, DeleteContestProblemQuery, c.ContestId, c.ProblemId)
	if err != nil {
		return pkg.HandlePgErr(err)
	}

	return nil
}

//go:embed sql/get_contest_problem.sql
var GetContestProblemQuery string

func (r *Repository) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error) {
	var contestProblem models.ContestProblem
	err := r.db.GetContext(ctx, &contestProblem, GetContestProblemQuery, c.ContestId, c.ProblemId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &contestProblem, nil
}

//go:embed sql/get_contest_problems.sql
var GetContestProblemsQuery string

func (r *Repository) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error) {
	contestProblems := make([]*models.ContestProblemsListItem, 0)
	err := r.db.SelectContext(ctx, &contestProblems, GetContestProblemsQuery, contestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return contestProblems, nil
}

//go:embed sql/list_contest_members.sql
var ListContestMembersQuery string

//go:embed sql/count_contest_members.sql
var CountContestMembersQuery string

func (r *Repository) ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) (*models.ParticipantsList, error) {
	var participants []*models.ContestMember
	err := r.db.SelectContext(ctx, &participants,
		ListContestMembersQuery,
		filter.ContestId,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, CountContestMembersQuery, filter.ContestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &models.ParticipantsList{
		Users: participants,
		Pagination: &models.Pagination{
			Total: models.Total(count, filter.PageSize),
			Page:  filter.Page,
		},
	}, nil
}

//go:embed sql/create_contest_member.sql
var CreateContestMemberQuery string

func (r *Repository) CreateContestMember(ctx context.Context, c *models.ContestMemberParams) error {
	_, err := r.db.ExecContext(ctx, CreateContestMemberQuery, c.ContestId, c.UserId, c.Role)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

//go:embed sql/get_contest_member.sql
var GetContestMemberQuery string

func (r *Repository) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMemberRecord, error) {
	var member models.ContestMemberRecord
	err := r.db.GetContext(ctx, &member, GetContestMemberQuery, c.ContestId, c.UserId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return &member, nil
}

//go:embed sql/delete_contest_member.sql
var DeleteContestMemberQuery string

func (r *Repository) DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, DeleteContestMemberQuery, contestId, userId)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}
