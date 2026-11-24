package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func safeInt32[T ~int | ~int64](v T) int32 {
	if int64(v) > math.MaxInt32 {
		return math.MaxInt32
	}
	if int64(v) < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

type ContestsRepo struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewContestsRepo(db *pgxpool.Pool) *ContestsRepo {
	return &ContestsRepo{
		pool:    db,
		queries: sqlc.New(db),
	}
}

func (r *ContestsRepo) CreateContest(ctx context.Context, params *contestsmodel.CreateContestParams) error {
	_, err := r.queries.CreateContest(ctx, sqlc.CreateContestParams{
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		Visibility:     params.Visibility,
		Title:          params.Title,
		Login:          params.Login,
		Description:    params.Description,
		StartTime:      params.StartTime,
		EndTime:        params.EndTime,
	})
	if err != nil {
		return dbError("create contest", err)
	}
	return nil
}

func (r *ContestsRepo) ListContestOwners(ctx context.Context, contestID uuid.UUID) ([]uuid.UUID, error) {
	owners, err := r.queries.ListContestOwners(ctx, contestID)
	if err != nil {
		return nil, dbError("list contest owners", err)
	}
	return owners, nil
}

func (r *ContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByID(ctx, id)
	if err != nil {
		return contestsmodel.Contest{}, dbError("get contest", err)
	}
	return mapContest(contest.Contest, contest.OrgLogin), nil
}

func (r *ContestsRepo) GetContestByLogin(ctx context.Context, orgID uuid.UUID, login string) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByLogin(ctx, sqlc.GetContestByLoginParams{
		OrganizationID: orgID,
		Lower:          login,
	})
	if err != nil {
		return contestsmodel.Contest{}, dbError("get contest by login", err)
	}
	return mapContest(contest.Contest, contest.OrgLogin), nil
}

func (r *ContestsRepo) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByOrgLoginAndContestLogin(ctx, sqlc.GetContestByOrgLoginAndContestLoginParams{
		Lower:   orgLogin,
		Lower_2: contestLogin,
	})
	if err != nil {
		return contestsmodel.Contest{}, dbError("get contest by org login and contest login", err)
	}
	return mapContest(contest.Contest, contest.OrgLogin), nil
}

func (r *ContestsRepo) UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error {
	s := c.Settings
	err := r.queries.UpdateContest(ctx, sqlc.UpdateContestParams{
		ID:                    c.ID,
		Login:                 c.Login,
		Title:                 c.Title,
		Place:                 c.Place,
		Description:           c.Description,
		Visibility:            c.Visibility,
		JoinPolicy:            s.JoinPolicy,
		ScoreboardDuring:      s.ScoreboardDuring,
		ScoreboardAfter:       s.ScoreboardAfter,
		SubmissionsDuring:     s.SubmissionsDuring,
		SubmissionsAfter:      s.SubmissionsAfter,
		SourcesDuring:         s.SourcesDuring,
		SourcesAfter:          s.SourcesAfter,
		ReportsDuring:         s.ReportsDuring,
		ReportsAfter:          s.ReportsAfter,
		TutorialsDuring:       s.TutorialsDuring,
		TutorialsAfter:        s.TutorialsAfter,
		FeedbackDuring:        s.FeedbackDuring,
		FeedbackAfter:         s.FeedbackAfter,
		FreezeStatus:          s.FreezeStatus,
		FreezeDurationSet:     s.FreezeDurationSet,
		FreezeDurationMinutes: s.FreezeDurationMinutes,
		FreezeHidesAttempts:   s.FreezeHidesAttempts,
		UnfreezeTimeSet:       s.UnfreezeTimeSet,
		UnfreezeTime:          s.UnfreezeTime,
		HideStatements:        s.HideStatements,
		AllowClarifications:   s.AllowClarifications,
		EnableDrafts:          s.EnableDrafts,
		EnableUpsolving:       s.EnableUpsolving,
		AllowVirtual:          s.AllowVirtual,
		Pretests:              s.Pretests,
		Languages:             s.Languages,
		StartTime:             c.StartTime,
		StartTimeSet:          c.StartTimeSet,
		EndTime:               c.EndTime,
		EndTimeSet:            c.EndTimeSet,
	})
	if err != nil {
		return dbError("update contest", err)
	}
	return nil
}

func (r *ContestsRepo) DeleteContest(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteContest(ctx, id)
	if err != nil {
		return dbError("delete contest", err)
	}
	return nil
}

func (r *ContestsRepo) ListContests(ctx context.Context, filter contestsmodel.ListContestsFilter) (*contestsmodel.ContestsList, error) {
	page, size, err := paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{OrganizationID: filter.OrganizationID, TeamID: filter.TeamID, Search: filter.Search, Visibility: filter.Visibility, SortBy: filter.SortBy, SortOrder: filter.SortOrder}, page, size)
}

func (r *ContestsRepo) listScopedContests(ctx context.Context, params sqlc.ScopeContestsParams, page, size int32) (*contestsmodel.ContestsList, error) {
	var err error
	if params.ViewerID, params.All, err = (Access{q: r.queries}).visibility(ctx); err != nil {
		return nil, err
	}
	params.PageSize, params.PageOffset = size, (int64(page)-1)*int64(size)
	items, total, err := scopeContests(ctx, r.queries, params)
	if err != nil {
		return nil, err
	}
	return &contestsmodel.ContestsList{Contests: items, Pagination: paging.NewPagination(page, size, total)}, nil
}

func (r *ContestsRepo) CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error {
	err := r.queries.AddContestProblem(ctx, sqlc.AddContestProblemParams{
		ContestID: c.ContestId, ProblemID: c.ProblemId, PackageID: c.PackageId,
	})
	return dbError("create contest problem", err)
}

func (r *ContestsRepo) ResolveContestProblemPackage(ctx context.Context, problemID, packageID uuid.UUID, allowLatest bool) (contestsmodel.PackageRef, error) {
	if packageID == uuid.Nil && !allowLatest {
		return contestsmodel.PackageRef{}, pkg.Wrap(pkg.ErrRequired, nil, "package is required")
	}
	var row sqlc.ProblemPackage
	var err error
	if packageID == uuid.Nil {
		row, err = r.queries.GetReadyPackage(ctx, problemID)
	} else {
		row, err = r.queries.GetProblemPackageByID(ctx, packageID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return contestsmodel.PackageRef{}, pkg.Wrap(pkg.ErrPackageNotReady, nil, "ready package not found")
	}
	if err != nil {
		return contestsmodel.PackageRef{}, dbError("resolve contest problem package", err)
	}
	return contestsmodel.PackageRef{ID: row.ID, ProblemID: row.ProblemID}, nil
}

func (r *ContestsRepo) DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error {
	err := r.queries.RemoveContestProblem(ctx, sqlc.RemoveContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return dbError("delete contest problem", err)
	}
	return nil
}

func (r *ContestsRepo) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error {
	q := r.queries

	// Temporary positions must exceed existing and requested ordinals to avoid uniqueness conflicts.
	maximum, err := q.GetMaxContestProblemOrdinal(ctx, contestId)
	if err != nil {
		return dbError("reorder contest problems", err)
	}
	for _, p := range problems {
		maximum = max(maximum, p.Position)
	}
	if int64(maximum)+int64(len(problems)) > math.MaxInt32 {
		return pkg.Wrap(pkg.ErrInvalid, nil, "problem ordinals exceed the supported range")
	}
	for i, p := range problems {
		tempOrdinal := maximum + safeInt32(i+1)
		err := q.UpdateContestProblemOrdinal(ctx, sqlc.UpdateContestProblemOrdinalParams{
			ContestID: contestId,
			ProblemID: p.ProblemID,
			Ordinal:   tempOrdinal,
		})
		if err != nil {
			return dbError("reorder contest problems", err)
		}
	}

	for _, p := range problems {
		err := q.UpdateContestProblemOrdinal(ctx, sqlc.UpdateContestProblemOrdinalParams{
			ContestID: contestId,
			ProblemID: p.ProblemID,
			Ordinal:   p.Position,
		})
		if err != nil {
			return dbError("reorder contest problems", err)
		}
	}

	return nil
}

func (r *ContestsRepo) GetContestProblem(ctx context.Context, c contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	row, err := r.queries.GetContestProblem(ctx, sqlc.GetContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return contestsmodel.ContestProblem{}, dbError("get contest problem", err)
	}
	return mapGetContestProblemRow(row), nil
}

func (r *ContestsRepo) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	rows, err := r.queries.ListContestProblems(ctx, contestId)
	if err != nil {
		return nil, dbError("get contest problems", err)
	}

	problems := make([]contestsmodel.ContestProblem, len(rows))
	for i, row := range rows {
		problems[i] = mapListContestProblemsRow(row)
	}

	return problems, nil
}

func (r *ContestsRepo) UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error {
	err := r.queries.UpdateContestProblemPackage(ctx, sqlc.UpdateContestProblemPackageParams{
		ContestID: contestId,
		ProblemID: problemId,
		PackageID: packageId,
	})
	if err != nil {
		return dbError("update contest problem package", err)
	}
	return nil
}

func (r *ContestsRepo) GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestTeam, error) {
	rows, err := r.queries.ListContestTeams(ctx, contestId)
	if err != nil {
		return nil, dbError("get contest teams", err)
	}

	teams := make([]contestsmodel.ContestTeam, len(rows))
	for i, row := range rows {
		teams[i] = mapContestTeam(row)
	}

	return teams, nil
}

func (r *ContestsRepo) CreateContestTeam(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
	role accessmodel.ContestRole,
) error {
	err := r.queries.AddContestTeam(ctx, sqlc.AddContestTeamParams{
		ContestID: contestID,
		TeamID:    teamID,
		Role:      role,
	})
	if err != nil {
		return dbError("create contest team", err)
	}

	return nil
}

func (r *ContestsRepo) UpdateContestTeamRole(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
	role accessmodel.ContestRole,
) error {
	err := r.queries.UpdateContestTeamRole(ctx, sqlc.UpdateContestTeamRoleParams{
		ContestID: contestID,
		TeamID:    teamID,
		Role:      role,
	})
	if err != nil {
		return dbError("update contest team role", err)
	}

	return nil
}

func (r *ContestsRepo) DeleteContestTeam(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
) error {
	err := r.queries.RemoveContestTeam(ctx, sqlc.RemoveContestTeamParams{
		ContestID: contestID,
		TeamID:    teamID,
	})
	if err != nil {
		return dbError("delete contest team", err)
	}

	return nil
}

func (r *ContestsRepo) ListContestMembers(ctx context.Context, filter contestsmodel.ParticipantsFilter) (*contestsmodel.ContestMembersList, error) {
	if r.pool != nil {
		return readSnapshot(ctx, r.pool, func(q *sqlc.Queries) (*contestsmodel.ContestMembersList, error) {
			return (&ContestsRepo{queries: q}).ListContestMembers(ctx, filter)
		})
	}
	rows, err := r.queries.ListContestMembers(ctx, sqlc.ListContestMembersParams{
		ContestID: filter.ContestId, PageLimit: filter.PageSize, PageOffset: Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, dbError("list contest members", err)
	}

	count, err := r.queries.CountContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, dbError("list contest members", err)
	}

	members := make([]contestsmodel.ContestMember, len(rows))
	for i, row := range rows {
		members[i] = mapListContestMembersRow(row)
	}

	return &contestsmodel.ContestMembersList{Members: members, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(count))}, nil
}

func (r *ContestsRepo) CreateContestMember(ctx context.Context, c *contestsmodel.CreateContestMemberParams) error {
	err := r.queries.AddContestMember(ctx, sqlc.AddContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
		Role:      c.Role,
	})
	if err != nil {
		return dbError("create contest member", err)
	}

	return nil
}

func (r *ContestsRepo) GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	member, err := r.queries.GetContestMember(ctx, sqlc.GetContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
	})
	if err != nil {
		return contestsmodel.ContestMember{}, dbError("get contest member", err)
	}

	return mapContestMember(member), nil
}

func (r *ContestsRepo) DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error {
	err := r.queries.RemoveContestMember(ctx, sqlc.RemoveContestMemberParams{
		UserID:    userId,
		ContestID: contestId,
	})
	if err != nil {
		return dbError("delete contest member", err)
	}
	return nil
}

func (r *ContestsRepo) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role accessmodel.ContestRole) error {
	err := r.queries.UpdateContestMemberRole(ctx, sqlc.UpdateContestMemberRoleParams{
		ContestID: contestId,
		UserID:    userId,
		Role:      role,
	})
	if err != nil {
		return dbError("update contest member", err)
	}

	return nil
}

func (r *ContestsRepo) ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]contestsmodel.UserContestMembership, error) {
	rows, err := r.queries.ListUserContestMemberships(ctx, userID)
	if err != nil {
		return nil, dbError("list user contest memberships", err)
	}

	memberships := make([]contestsmodel.UserContestMembership, len(rows))
	for i, row := range rows {
		memberships[i] = contestsmodel.UserContestMembership{
			ContestID: row.ContestID,
			Role:      row.Role,
		}
	}
	return memberships, nil
}

func (r *ContestsRepo) AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role accessmodel.ContestRole) error {
	err := r.queries.AddContestMemberIfNotExists(ctx, sqlc.AddContestMemberIfNotExistsParams{
		ContestID: contestID,
		UserID:    userID,
		Role:      role,
	})
	if err != nil {
		return dbError("add contest member if not exists", err)
	}
	return nil
}

func mapContest(c sqlc.Contest, orgLogin string) contestsmodel.Contest {
	return contestsmodel.Contest{
		ID:                c.ID,
		OrganizationID:    c.OrganizationID,
		OrganizationLogin: orgLogin,
		Visibility:        c.Visibility,
		Title:             c.Title,
		Place:             c.Place,
		Login:             c.Login,
		Description:       c.Description,
		Settings: contestsmodel.ContestSettings{
			JoinPolicy:            c.JoinPolicy,
			Scoreboard:            contestsmodel.Phased[contestsmodel.ContestScope]{During: c.ScoreboardDuring, After: c.ScoreboardAfter},
			Submissions:           contestsmodel.Phased[contestsmodel.ContestScope]{During: c.SubmissionsDuring, After: c.SubmissionsAfter},
			Sources:               contestsmodel.Phased[contestsmodel.ContestScope]{During: c.SourcesDuring, After: c.SourcesAfter},
			Reports:               contestsmodel.Phased[contestsmodel.ContestScope]{During: c.ReportsDuring, After: c.ReportsAfter},
			Tutorials:             contestsmodel.Phased[contestsmodel.ContestScope]{During: c.TutorialsDuring, After: c.TutorialsAfter},
			Feedback:              contestsmodel.Phased[contestsmodel.FeedbackLevel]{During: c.FeedbackDuring, After: c.FeedbackAfter},
			FreezeStatus:          c.FreezeStatus,
			FreezeDurationMinutes: c.FreezeDurationMinutes,
			FreezeStartedAt:       c.FreezeStartedAt,
			FreezeHidesAttempts:   c.FreezeHidesAttempts,
			UnfreezeTime:          c.UnfreezeTime,
			HideStatements:        c.HideStatements,
			AllowClarifications:   c.AllowClarifications,
			EnableDrafts:          c.EnableDrafts,
			EnableUpsolving:       c.EnableUpsolving,
			AllowVirtual:          c.AllowVirtual,
			Pretests:              c.Pretests,
			Languages:             c.Languages,
		},
		StartTime: c.StartTime,
		EndTime:   c.EndTime,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func mapGetContestProblemRow(c sqlc.GetContestProblemRow) contestsmodel.ContestProblem {
	return contestsmodel.ContestProblem{
		Type: c.Type, TimeLimitMs: int(c.TimeLimitMs), MemoryLimitMb: int(c.MemoryLimitMb), InputFile: c.InputFile, OutputFile: c.OutputFile, UpdatedAt: c.ProblemUpdatedAt,
		ContestID:  c.ContestID,
		ProblemID:  c.ProblemID,
		PackageID:  c.PackageID,
		Ordinal:    int(c.Ordinal),
		Title:      c.Title,
		ShortName:  c.ShortName,
		Visibility: string(c.Visibility),
		CreatedAt:  c.CreatedAt,
	}
}

func mapListContestProblemsRow(c sqlc.ListContestProblemsRow) contestsmodel.ContestProblem {
	return contestsmodel.ContestProblem{
		Type: c.Type, TimeLimitMs: int(c.TimeLimitMs), MemoryLimitMb: int(c.MemoryLimitMb), InputFile: c.InputFile, OutputFile: c.OutputFile, UpdatedAt: c.ProblemUpdatedAt,
		ContestID:  c.ContestID,
		ProblemID:  c.ProblemID,
		PackageID:  c.PackageID,
		Ordinal:    int(c.Ordinal),
		Title:      c.Title,
		ShortName:  c.ShortName,
		Visibility: string(c.Visibility),
		CreatedAt:  c.CreatedAt,
	}
}

func mapContestMember(c sqlc.GetContestMemberRow) contestsmodel.ContestMember {
	return contestsmodel.ContestMember{
		Username:    c.Username,
		UserID:      c.UserID,
		ContestID:   c.ContestID,
		ContestRole: accessmodel.ContestRole(c.Role),
	}
}

func mapContestTeam(c sqlc.ListContestTeamsRow) contestsmodel.ContestTeam {
	return contestsmodel.ContestTeam{
		ContestID: c.ContestID,
		TeamID:    c.TeamID,
		Role:      c.Role,
		TeamName:  c.TeamName,
		TeamSlug:  c.TeamSlug,
		CreatedAt: c.CreatedAt,
	}
}

func (r *ContestsRepo) ListContestParticipants(ctx context.Context, contestID uuid.UUID) (map[uuid.UUID]string, error) {
	rows, err := r.queries.ListContestParticipants(ctx, contestID)
	if err != nil {
		return nil, dbError("list contest participants", err)
	}
	users := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		users[row.UserID] = row.Username
	}
	return users, nil
}

func (r *ContestsRepo) GetContestScoreboardSubmissions(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	rows, err := r.queries.GetContestScoreboardSubmissions(ctx, contestID)
	if err != nil {
		return nil, dbError("get contest scoreboard submissions", err)
	}
	subs := make([]contestsmodel.ScoreboardSubmission, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.ScoreboardSubmission{UserID: *row.OwnerID, ProblemID: *row.ProblemID,
			SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: row.State, CreatedAt: row.CreatedAt}}
	}
	return subs, nil
}

func (r *ContestsRepo) GetUserScoreboardSubmissions(ctx context.Context, contestID, userID uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	rows, err := r.queries.GetUserScoreboardSubmissions(ctx, sqlc.GetUserScoreboardSubmissionsParams{ContestID: contestID, OwnerID: userID})
	if err != nil {
		return nil, dbError("get user scoreboard submissions", err)
	}
	subs := make([]contestsmodel.ScoreboardSubmission, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.ScoreboardSubmission{UserID: *row.OwnerID, ProblemID: *row.ProblemID,
			SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: row.State, CreatedAt: row.CreatedAt}}
	}
	return subs, nil
}

func (r *ContestsRepo) CreateContestUserProblemBlock(ctx context.Context, params *contestsmodel.CreateContestUserProblemBlockParams) error {
	createdBy := params.CreatedBy

	err := r.queries.CreateContestUserProblemBlock(ctx, sqlc.CreateContestUserProblemBlockParams{
		ContestID: params.ContestID,
		UserID:    params.UserID,
		ProblemID: params.ProblemID,
		Reason:    params.Reason,
		CreatedBy: createdBy,
	})
	if err != nil {
		return dbError("create contest user problem block", err)
	}
	return nil
}

func (r *ContestsRepo) DeleteContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) error {
	err := r.queries.DeleteContestUserProblemBlock(ctx, sqlc.DeleteContestUserProblemBlockParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
	})
	if err != nil {
		return dbError("delete contest user problem block", err)
	}
	return nil
}

func (r *ContestsRepo) GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestUserProblemBlock, error) {
	row, err := r.queries.GetContestUserProblemBlock(ctx, sqlc.GetContestUserProblemBlockParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
	})
	if err != nil {
		return nil, dbError("get contest user problem block", err)
	}

	createdBy := row.CreatedBy

	return &contestsmodel.ContestUserProblemBlock{
		ContestID: row.ContestID,
		UserID:    row.UserID,
		ProblemID: row.ProblemID,
		Reason:    row.Reason,
		CreatedBy: createdBy,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *ContestsRepo) ListContestUserProblemBlocks(ctx context.Context, contestID uuid.UUID, userID *uuid.UUID) ([]contestsmodel.ContestUserProblemBlock, error) {
	rows, err := r.queries.ListContestUserProblemBlocks(ctx, sqlc.ListContestUserProblemBlocksParams{
		ContestID: contestID,
		UserID:    userID,
	})
	if err != nil {
		return nil, dbError("list contest user problem blocks", err)
	}

	blocks := make([]contestsmodel.ContestUserProblemBlock, len(rows))
	for i, row := range rows {
		createdBy := row.CreatedBy
		blocks[i] = contestsmodel.ContestUserProblemBlock{
			ContestID: row.ContestID,
			UserID:    row.UserID,
			ProblemID: row.ProblemID,
			Reason:    row.Reason,
			CreatedBy: createdBy,
			CreatedAt: row.CreatedAt,
		}
	}
	return blocks, nil
}

func (r *ContestsRepo) CreateContestJoinRequest(ctx context.Context, input *contestsmodel.CreateContestJoinRequestInput) (*contestsmodel.ContestJoinRequest, error) {
	id := uuid.New()
	row, err := r.queries.CreateContestJoinRequest(ctx, sqlc.CreateContestJoinRequestParams{
		ID:        id,
		ContestID: input.ContestID,
		UserID:    input.UserID,
		Message:   input.Message,
	})
	if err != nil {
		return nil, dbError("create contest join request", err)
	}

	return r.GetContestJoinRequestByID(ctx, row.ID)
}

func (r *ContestsRepo) GetContestJoinRequestByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestJoinRequest, error) {
	row, err := r.queries.GetContestJoinRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, dbError("get contest join request by id", err)
	}

	var email string
	if row.Email != nil {
		email = *row.Email
	}

	reviewedBy := row.ReviewedBy

	return &contestsmodel.ContestJoinRequest{
		ID:                row.ID,
		ContestID:         row.ContestID,
		ContestTitle:      row.ContestTitle,
		ContestLogin:      row.ContestLogin,
		OrganizationLogin: row.OrganizationLogin,
		UserID:            row.UserID,
		Username:          row.Username,
		Email:             email,
		Message:           row.Message,
		Status:            organizationsmodel.RequestStatus(row.Status),
		ReviewedBy:        reviewedBy,
		ReviewerUsername:  row.ReviewerUsername,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (r *ContestsRepo) GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*contestsmodel.ContestJoinRequest, error) {
	row, err := r.queries.GetPendingContestJoinRequest(ctx, sqlc.GetPendingContestJoinRequestParams{
		ContestID: contestID,
		UserID:    userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, dbError("get pending contest join request", err)
	}

	return r.GetContestJoinRequestByID(ctx, row.ID)
}

//nolint:dupl // Mirrors ListJoinRequests for a different request type.
func (r *ContestsRepo) ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]contestsmodel.ContestJoinRequest, error) {
	rows, err := r.queries.ListContestJoinRequests(ctx, sqlc.ListContestJoinRequestsParams{
		ContestID: contestID,
		Status:    status,
	})
	if err != nil {
		return nil, dbError("list contest join requests", err)
	}

	requests := make([]contestsmodel.ContestJoinRequest, len(rows))
	for i, row := range rows {
		var email string
		if row.Email != nil {
			email = *row.Email
		}
		reviewedBy := row.ReviewedBy
		requests[i] = contestsmodel.ContestJoinRequest{
			ID:               row.ID,
			ContestID:        row.ContestID,
			UserID:           row.UserID,
			Username:         row.Username,
			Email:            email,
			Message:          row.Message,
			Status:           organizationsmodel.RequestStatus(row.Status),
			ReviewedBy:       reviewedBy,
			ReviewerUsername: row.ReviewerUsername,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		}
	}
	return requests, nil
}

func (r *ContestsRepo) ListUserContestJoinRequests(ctx context.Context, userID uuid.UUID) ([]contestsmodel.ContestJoinRequest, error) {
	rows, err := r.queries.ListUserContestJoinRequests(ctx, userID)
	if err != nil {
		return nil, dbError("list user contest join requests", err)
	}

	requests := make([]contestsmodel.ContestJoinRequest, len(rows))
	for i, row := range rows {
		requests[i] = contestsmodel.ContestJoinRequest{
			ID:                row.ID,
			ContestID:         row.ContestID,
			ContestTitle:      row.ContestTitle,
			ContestLogin:      row.ContestLogin,
			OrganizationLogin: row.OrganizationLogin,
			UserID:            row.UserID,
			Message:           row.Message,
			Status:            organizationsmodel.RequestStatus(row.Status),
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		}
	}
	return requests, nil
}

func (r *ContestsRepo) UpdateContestJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error {
	pgReviewedBy := reviewedBy
	affected, err := r.queries.UpdateContestJoinRequestStatus(ctx, sqlc.UpdateContestJoinRequestStatusParams{
		ID:         id,
		Status:     string(status),
		ReviewedBy: pgReviewedBy,
	})
	if err != nil {
		return dbError("update contest join request status", err)
	}
	if affected != 1 {
		return pkg.Wrap(pkg.ErrRequestResolved, nil, "request is no longer pending")
	}
	return nil
}

func mapListContestMembersRow(c sqlc.ListContestMembersRow) contestsmodel.ContestMember {
	return contestsmodel.ContestMember{
		UserID:      c.UserID,
		ContestID:   c.ContestID,
		Username:    c.Username,
		ContestRole: c.Role,
		CreatedAt:   c.CreatedAt,
	}
}

func (r *ContestsRepo) GetContestTeam(ctx context.Context, contestID, teamID uuid.UUID) (contestsmodel.ContestTeam, error) {
	row, err := r.queries.GetContestTeam(ctx, sqlc.GetContestTeamParams{ContestID: contestID, TeamID: teamID})
	if err != nil {
		return contestsmodel.ContestTeam{}, dbError("get contest team", err)
	}
	return contestsmodel.ContestTeam{ContestID: row.ContestID, TeamID: row.TeamID, Role: row.Role, CreatedAt: row.CreatedAt}, nil
}

func (r *ContestsRepo) CreateVirtualRun(ctx context.Context, contestID uuid.UUID, run contestsmodel.VirtualRun) error {
	err := r.queries.CreateVirtualRun(ctx, sqlc.CreateVirtualRunParams{ContestID: contestID, UserID: run.UserID, StartedAt: run.StartedAt, EndsAt: run.EndsAt})
	return dbError("create virtual run", err)
}

// FinishVirtualRun ends an active run at the given time; a run already over is left as is.
func (r *ContestsRepo) FinishVirtualRun(ctx context.Context, contestID, userID uuid.UUID, at time.Time) error {
	err := r.queries.FinishVirtualRun(ctx, sqlc.FinishVirtualRunParams{ContestID: contestID, UserID: userID, FinishedAt: at})
	return dbError("finish virtual run", err)
}

func (r *ContestsRepo) ListVirtualRuns(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.VirtualRun, error) {
	rows, err := r.queries.ListVirtualRuns(ctx, contestID)
	if err != nil {
		return nil, dbError("list virtual runs", err)
	}
	runs := make([]contestsmodel.VirtualRun, len(rows))
	for i, row := range rows {
		runs[i] = contestsmodel.VirtualRun{UserID: row.UserID, Username: row.Username, StartedAt: row.StartedAt, EndsAt: row.EndsAt, FinishedAt: row.FinishedAt}
	}
	return runs, nil
}

func (r *ContestsRepo) GetVirtualScoreboardSubmissions(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	rows, err := r.queries.GetVirtualScoreboardSubmissions(ctx, contestID)
	if err != nil {
		return nil, dbError("get virtual scoreboard submissions", err)
	}
	subs := make([]contestsmodel.ScoreboardSubmission, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.ScoreboardSubmission{UserID: *row.OwnerID, ProblemID: *row.ProblemID,
			SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: row.State, CreatedAt: row.CreatedAt}}
	}
	return subs, nil
}
