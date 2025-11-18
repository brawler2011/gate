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

func (r *Repository) CreateContest(ctx context.Context, contestCreation models.ContestCreation) (uuid.UUID, error) {
	const op = "Repository.CreateContest"

	rows, err := r.db.QueryxContext(ctx, CreateContestQuery, contestCreation.Title)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	defer rows.Close()
	var id uuid.UUID
	rows.Next()
	err = rows.Scan(&id)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	return id, nil
}

//go:embed sql/get_contest.sql
var GetContestQuery string

func (r *Repository) GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error) {
	const op = "Repository.GetContest"

	var contest models.Contest
	err := r.db.GetContext(ctx, &contest, GetContestQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}
	return &contest, nil
}

//go:embed sql/update_contest.sql
var UpdateContestQuery string

func (r *Repository) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	const op = "Repository.UpdateContest"

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
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/delete_contest.sql
var DeleteContestQuery string

func (r *Repository) DeleteContest(ctx context.Context, id uuid.UUID) error {
	const op = "Repository.DeleteContest"

	_, err := r.db.ExecContext(ctx, DeleteContestQuery, id)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/list_contests.sql
var ListContestsQuery string

//go:embed sql/count_contests.sql
var CountContestsQuery string

func (r *Repository) ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error) {
	const op = "Repository.ListContests"

	contests := make([]*models.Contest, 0)
	err := r.db.SelectContext(ctx, &contests, ListContestsQuery,
		filter.OwnerId,
		filter.Search,
		filter.Descending,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, CountContestsQuery,
		filter.OwnerId,
		filter.Search,
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
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
	const op = "Repository.ContestProblemCreation"

	_, err := r.db.ExecContext(ctx, CreateContestProblemQuery, c.ProblemId, c.ContestId)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/delete_contest_problem.sql
var DeleteContestProblemQuery string

func (r *Repository) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	const op = "Repository.DeleteContestProblem"

	_, err := r.db.ExecContext(ctx, DeleteContestProblemQuery, c.ContestId, c.ProblemId)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/get_contest_problem.sql
var GetContestProblemQuery string

func (r *Repository) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error) {
	const op = "Repository.GetContestProblem"

	var contestProblem models.ContestProblem
	err := r.db.GetContext(ctx, &contestProblem, GetContestProblemQuery, c.ContestId, c.ProblemId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &contestProblem, nil
}

//go:embed sql/get_contest_problems.sql
var GetContestProblemsQuery string

func (r *Repository) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error) {
	const op = "Repository.GetContestProblems"

	contestProblems := make([]*models.ContestProblemsListItem, 0)
	err := r.db.SelectContext(ctx, &contestProblems, GetContestProblemsQuery, contestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return contestProblems, nil
}

//go:embed sql/create_participant.sql
var CreateParticipantQuery string

func (r *Repository) CreateParticipant(ctx context.Context, c models.ParticipantCreation) error {
	const op = "Repository.CreateParticipant"

	_, err := r.db.ExecContext(ctx, CreateParticipantQuery, c.UserId, c.ContestId)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/delete_participant.sql
var DeleteParticipantQuery string

func (r *Repository) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	const op = "Repository.DeleteParticipant"

	_, err := r.db.ExecContext(ctx, DeleteParticipantQuery, c.UserId, c.ContestId)
	if err != nil {
		return pkg.HandlePgErr(err, op)
	}

	return nil
}

//go:embed sql/list_participants.sql
var ListParticipantsQuery string

//go:embed sql/count_participants.sql
var CountParticipantsQuery string

func (r *Repository) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.UsersList, error) {
	const op = "Repository.ListParticipants"

	var participants []*models.User
	err := r.db.SelectContext(ctx, &participants,
		ListParticipantsQuery,
		filter.ContestId,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	var count int64
	err = r.db.GetContext(ctx, &count, CountParticipantsQuery, filter.ContestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &models.UsersList{
		Users: participants,
		Pagination: models.Pagination{
			Total: models.Total(count, filter.PageSize),
			Page:  filter.Page,
		},
	}, nil
}

//go:embed sql/get_contest_member.sql
var GetContestMemberQuery string

func (r *Repository) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMember, error) {
	const op = "Repository.GetContestMember"

	var member models.ContestMember
	err := r.db.GetContext(ctx, &member, GetContestMemberQuery, c.ContestId, c.UserId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &member, nil
}
