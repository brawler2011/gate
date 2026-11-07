package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
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
	settingsJSON, err := marshalJSON(params.Settings)
	if err != nil {
		return err
	}

	_, err = r.queries.CreateContest(ctx, sqlc.CreateContestParams{
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		OwnerID:        params.OwnerID,
		Visibility:     params.Visibility,
		Title:          params.Title,
		Login:          params.Login,
		Description:    params.Description,
		Settings:       settingsJSON,
		StartTime:      params.StartTime,
		EndTime:        params.EndTime,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByID(ctx, id)
	if err != nil {
		return contestsmodel.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByIDRow(contest), nil
}

func (r *ContestsRepo) GetContestByLogin(ctx context.Context, orgID uuid.UUID, login string) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByLogin(ctx, sqlc.GetContestByLoginParams{
		OrganizationID: orgID,
		Lower:          login,
	})
	if err != nil {
		return contestsmodel.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByLoginRow(contest), nil
}

func (r *ContestsRepo) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (contestsmodel.Contest, error) {
	contest, err := r.queries.GetContestByOrgLoginAndContestLogin(ctx, sqlc.GetContestByOrgLoginAndContestLoginParams{
		Lower:   orgLogin,
		Lower_2: contestLogin,
	})
	if err != nil {
		return contestsmodel.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByOrgLoginAndContestLoginRow(contest), nil
}

func (r *ContestsRepo) UpdateContest(ctx context.Context, c contestsmodel.ContestUpdateInput) error {
	var settingsJSON []byte
	var err error

	if c.Settings != nil {
		settingsJSON, err = marshalJSON(*c.Settings)
		if err != nil {
			return err
		}
	}

	err = r.queries.UpdateContest(ctx, sqlc.UpdateContestParams{
		ID:           c.ID,
		Login:        c.Login,
		Title:        c.Title,
		Description:  c.Description,
		Visibility:   c.Visibility,
		Settings:     settingsJSON,
		StartTime:    c.StartTime,
		StartTimeSet: c.StartTimeSet,
		EndTime:      c.EndTime,
		EndTimeSet:   c.EndTimeSet,
		OwnerID:      c.OwnerID,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) DeleteContest(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteContest(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) ListOrganizationContests(ctx context.Context, orgLogin, search string, page, pageSize int32) (*contestsmodel.ContestsList, error) {
	org, err := r.queries.GetOrganizationByLogin(ctx, orgLogin)
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{OrganizationID: &org.ID, Search: search}, page, pageSize)
}
func (r *ContestsRepo) ListAdminContests(ctx context.Context, filter contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error) {
	var visibility string
	if filter.Visibility != nil {
		visibility = string(*filter.Visibility)
	}
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{Search: filter.Search, Visibility: visibility, SortBy: filter.SortBy, SortOrder: filter.SortOrder}, filter.Page, filter.PageSize)
}
func (r *ContestsRepo) ListUserContests(ctx context.Context, filter contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error) {
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{Search: filter.Search, SortBy: filter.SortBy, SortOrder: filter.SortOrder}, filter.Page, filter.PageSize)
}
func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error) {
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{OrganizationID: filter.OrganizationID, Search: filter.Search, SortBy: filter.SortBy, SortOrder: filter.SortOrder}, filter.Page, filter.PageSize)
}
func (r *ContestsRepo) ListPublicContests(ctx context.Context, filter contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error) {
	page, size, err := paging.Normalize(filter.Page, filter.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return r.listScopedContests(ctx, sqlc.ScopeContestsParams{Search: filter.Search, Visibility: string(contestsmodel.ContestVisibilityPublic), SortBy: filter.SortBy, SortOrder: filter.SortOrder}, page, size)
}
func (r *ContestsRepo) listScopedContests(ctx context.Context, params sqlc.ScopeContestsParams, page, size int32) (*contestsmodel.ContestsList, error) {
	viewer := accessmodel.UserFromContext(ctx)
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	params.ViewerID, params.Now = &viewer.Id, accessmodel.ReadTime(ctx)
	params.PageSize, params.PageOffset = size, (int64(page)-1)*int64(size)
	items, total, err := scopeContests(ctx, r.queries, params)
	if err != nil {
		return nil, err
	}
	return &contestsmodel.ContestsList{Contests: items, Pagination: paging.NewPagination(page, size, total)}, nil
}

func (r *ContestsRepo) CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error {
	packageId, err := r.contestProblemPackage(ctx, c.ProblemId, c.PackageId, true)
	if err != nil {
		return err
	}

	err = r.queries.AddContestProblem(ctx, sqlc.AddContestProblemParams{
		ContestID: c.ContestId, ProblemID: c.ProblemId, PackageID: packageId,
	})
	return HandlePgErr(err)
}

// contestProblemPackage resolves and validates the immutable package to pin.
func (r *ContestsRepo) contestProblemPackage(ctx context.Context, problemID, packageID uuid.UUID, allowLatest bool) (uuid.UUID, error) {
	if packageID == uuid.Nil && !allowLatest {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "package is required")
	}
	var row sqlc.ProblemPackage
	var err error
	if packageID == uuid.Nil {
		row, err = r.queries.GetReadyPackage(ctx, problemID)
	} else {
		row, err = r.queries.GetProblemPackageByID(ctx, packageID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "ready package not found")
	}
	if err != nil {
		return uuid.Nil, HandlePgErr(err)
	}
	if row.ProblemID != problemID || row.Status != "ready" {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "package must belong to the problem and be ready")
	}
	return row.ID, nil
}

func (r *ContestsRepo) DeleteContestProblem(ctx context.Context, c contestsmodel.ContestProblemDeletion) error {
	err := r.queries.RemoveContestProblem(ctx, sqlc.RemoveContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []contestsmodel.ContestProblemReorderItem) error {
	q := r.queries

	// Reserve temporary positions above both existing and requested ordinals.
	maximum, err := q.GetMaxContestProblemOrdinal(ctx, contestId)
	if err != nil {
		return HandlePgErr(err)
	}
	for _, p := range problems {
		maximum = max(maximum, p.Position)
	}
	if int64(maximum)+int64(len(problems)) > math.MaxInt32 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "problem ordinals exceed the supported range")
	}
	for i, p := range problems {
		tempOrdinal := maximum + safeInt32(i+1)
		err := q.UpdateContestProblemOrdinal(ctx, sqlc.UpdateContestProblemOrdinalParams{
			ContestID: contestId,
			ProblemID: p.ProblemID,
			Ordinal:   tempOrdinal,
		})
		if err != nil {
			return HandlePgErr(err)
		}
	}

	// Step 2: Assign final ordinals
	for _, p := range problems {
		err := q.UpdateContestProblemOrdinal(ctx, sqlc.UpdateContestProblemOrdinalParams{
			ContestID: contestId,
			ProblemID: p.ProblemID,
			Ordinal:   p.Position,
		})
		if err != nil {
			return HandlePgErr(err)
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
		return contestsmodel.ContestProblem{}, HandlePgErr(err)
	}
	return mapGetContestProblemRow(row), nil
}

func (r *ContestsRepo) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	rows, err := r.queries.ListContestProblems(ctx, contestId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	problems := make([]contestsmodel.ContestProblem, len(rows))
	for i, row := range rows {
		problems[i] = mapListContestProblemsRow(row)
	}

	return problems, nil
}

func (r *ContestsRepo) UpdateContestProblemPackage(ctx context.Context, contestId, problemId, packageId uuid.UUID) error {
	packageId, err := r.contestProblemPackage(ctx, problemId, packageId, false)
	if err != nil {
		return err
	}

	err = r.queries.UpdateContestProblemPackage(ctx, sqlc.UpdateContestProblemPackageParams{
		ContestID: contestId,
		ProblemID: problemId,
		PackageID: packageId,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]contestsmodel.ContestTeam, error) {
	rows, err := r.queries.ListContestTeams(ctx, contestId)
	if err != nil {
		return nil, HandlePgErr(err)
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
	role contestsmodel.ContestRole,
) error {
	err := r.queries.AddContestTeam(ctx, sqlc.AddContestTeamParams{
		ContestID: contestID,
		TeamID:    teamID,
		Role:      role,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *ContestsRepo) UpdateContestTeamRole(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
	role contestsmodel.ContestRole,
) error {
	err := r.queries.UpdateContestTeamRole(ctx, sqlc.UpdateContestTeamRoleParams{
		ContestID: contestID,
		TeamID:    teamID,
		Role:      role,
	})
	if err != nil {
		return HandlePgErr(err)
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
		return HandlePgErr(err)
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
		return nil, HandlePgErr(err)
	}

	count, err := r.queries.CountContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, HandlePgErr(err)
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
		return HandlePgErr(err)
	}

	return nil
}

func (r *ContestsRepo) GetContestMember(ctx context.Context, c *contestsmodel.ContestPermissionGet) (contestsmodel.ContestMember, error) {
	member, err := r.queries.GetContestMember(ctx, sqlc.GetContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
	})
	if err != nil {
		return contestsmodel.ContestMember{}, HandlePgErr(err)
	}

	return mapContestMember(member), nil
}

func (r *ContestsRepo) DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error {
	err := r.queries.RemoveContestMember(ctx, sqlc.RemoveContestMemberParams{
		UserID:    userId,
		ContestID: contestId,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role contestsmodel.ContestRole) error {
	err := r.queries.UpdateContestMemberRole(ctx, sqlc.UpdateContestMemberRoleParams{
		ContestID: contestId,
		UserID:    userId,
		Role:      role,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *ContestsRepo) ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]contestsmodel.UserContestMembership, error) {
	rows, err := r.queries.ListUserContestMemberships(ctx, userID)
	if err != nil {
		return nil, HandlePgErr(err)
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

func (r *ContestsRepo) AddContestMemberIfNotExists(ctx context.Context, contestID, userID uuid.UUID, role string) error {
	err := r.queries.AddContestMemberIfNotExists(ctx, sqlc.AddContestMemberIfNotExistsParams{
		ContestID: contestID,
		UserID:    userID,
		Role:      role,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func mapContestFields(
	id uuid.UUID,
	orgID uuid.UUID,
	orgLogin string,
	ownerID *uuid.UUID,
	visibility string,
	title string,
	login string,
	description string,
	settingsBytes []byte,
	startTime *time.Time,
	endTime *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
) contestsmodel.Contest {
	var settings map[string]interface{}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &settings)
	}

	return contestsmodel.Contest{
		ID:                id,
		OrganizationID:    orgID,
		OrganizationLogin: orgLogin,
		OwnerID:           ownerID,
		Visibility:        visibility,
		Title:             title,
		Login:             login,
		Description:       description,
		Settings:          settings,
		StartTime:         startTime,
		EndTime:           endTime,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}
}

func mapGetContestByIDRow(c sqlc.GetContestByIDRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestByLoginRow(c sqlc.GetContestByLoginRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestByOrgLoginAndContestLoginRow(c sqlc.GetContestByOrgLoginAndContestLoginRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestProblemRow(c sqlc.GetContestProblemRow) contestsmodel.ContestProblem {
	return contestsmodel.ContestProblem{
		TimeLimitMs: int(c.TimeLimitMs), MemoryLimitMb: int(c.MemoryLimitMb), UpdatedAt: c.ProblemUpdatedAt,
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
		TimeLimitMs: int(c.TimeLimitMs), MemoryLimitMb: int(c.MemoryLimitMb), UpdatedAt: c.ProblemUpdatedAt,
		ContestID:  c.ContestID,
		ProblemID:  c.ProblemID,
		PackageID:  c.PackageID,
		Ordinal:    int(c.Ordinal),
		Title:      c.Title,
		ShortName:  c.ShortName,
		Visibility: string(c.Visibility),
		PackageURL: c.PackageUrl,
		CreatedAt:  c.CreatedAt,
	}
}

func mapContestMember(c sqlc.GetContestMemberRow) contestsmodel.ContestMember {
	return contestsmodel.ContestMember{
		Username:    c.Username,
		UserID:      c.UserID,
		ContestID:   c.ContestID,
		ContestRole: contestsmodel.ContestRole(c.Role),
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

func marshalJSON(v map[string]interface{}) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

func (r *ContestsRepo) ListDashboardContests(ctx context.Context, userID uuid.UUID, limit int32) ([]contestsmodel.DashboardContest, error) {
	rows, err := r.queries.ListDashboardContests(ctx, sqlc.ListDashboardContestsParams{
		UserID: userID, ViewerID: readViewerID(ctx), Now: accessmodel.ReadTime(ctx), PageSize: limit,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]contestsmodel.DashboardContest, len(rows))
	for i, row := range rows {
		contests[i] = mapDashboardContest(row)
	}

	return contests, nil
}

func (r *ContestsRepo) UpsertContestProblemResult(ctx context.Context, params *contestsmodel.UpsertContestProblemResultParams) error {
	firstACTime := params.FirstACTime

	err := r.queries.UpsertContestProblemResult(ctx, sqlc.UpsertContestProblemResultParams{
		ContestID:      params.ContestID,
		UserID:         params.UserID,
		ProblemID:      params.ProblemID,
		Solved:         params.Solved,
		FailedAttempts: params.FailedAttempts,
		FirstAcTime:    firstACTime,
		TimeMinutes:    params.TimeMinutes,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*contestsmodel.ContestProblemResult, error) {
	row, err := r.queries.GetContestProblemResult(ctx, sqlc.GetContestProblemResultParams{
		ContestID: contestID,
		UserID:    userID,
		ProblemID: problemID,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	firstAC := row.FirstAcTime

	return &contestsmodel.ContestProblemResult{
		ContestID:      row.ContestID,
		UserID:         row.UserID,
		ProblemID:      row.ProblemID,
		Solved:         row.Solved,
		FailedAttempts: row.FailedAttempts,
		FirstACTime:    firstAC,
		TimeMinutes:    row.TimeMinutes,
	}, nil
}

func (r *ContestsRepo) GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ContestProblemResult, map[uuid.UUID]string, error) {
	rows, err := r.queries.GetContestScoreboardFromStandings(ctx, contestID)
	if err != nil {
		return nil, nil, HandlePgErr(err)
	}

	userMap := make(map[uuid.UUID]string)
	results := make([]contestsmodel.ContestProblemResult, 0, len(rows))

	for _, row := range rows {
		userMap[row.UserID] = row.Username
		if row.ProblemID != nil {
			probID := *row.ProblemID
			var solved bool
			if row.Solved != nil {
				solved = *row.Solved
			}
			var failedAttempts int32
			if row.FailedAttempts != nil {
				failedAttempts = *row.FailedAttempts
			}
			firstAC := row.FirstAcTime

			results = append(results, contestsmodel.ContestProblemResult{
				ContestID:      contestID,
				UserID:         row.UserID,
				ProblemID:      probID,
				Solved:         solved,
				FailedAttempts: failedAttempts,
				FirstACTime:    firstAC,
				TimeMinutes:    row.TimeMinutes,
			})
		}
	}

	return results, userMap, nil
}

func (r *ContestsRepo) GetContestScoreboardSubmissions(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	rows, err := r.queries.GetContestScoreboardSubmissions(ctx, &contestID)
	if err != nil {
		return nil, HandlePgErr(err)
	}
	subs := make([]contestsmodel.ScoreboardSubmission, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.ScoreboardSubmission{UserID: *row.OwnerID, ProblemID: *row.ProblemID,
			SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: row.State, CreatedAt: row.CreatedAt}}
	}
	return subs, nil
}

func (r *ContestsRepo) GetUserScoreboardSubmissions(ctx context.Context, contestID, userID uuid.UUID) ([]contestsmodel.ScoreboardSubmission, error) {
	rows, err := r.queries.GetUserScoreboardSubmissions(ctx, sqlc.GetUserScoreboardSubmissionsParams{ContestID: &contestID, OwnerID: &userID})
	if err != nil {
		return nil, HandlePgErr(err)
	}
	subs := make([]contestsmodel.ScoreboardSubmission, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.ScoreboardSubmission{UserID: *row.OwnerID, ProblemID: *row.ProblemID,
			SubmissionForScoreboard: contestsmodel.SubmissionForScoreboard{State: row.State, CreatedAt: row.CreatedAt}}
	}
	return subs, nil
}

func (r *ContestsRepo) GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error) {
	rows, err := r.queries.GetSubmissionsForScoreboard(ctx, sqlc.GetSubmissionsForScoreboardParams{
		ContestID: &contestID,
		OwnerID:   &userID,
		ProblemID: &problemID,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	subs := make([]contestsmodel.SubmissionForScoreboard, len(rows))
	for i, row := range rows {
		subs[i] = contestsmodel.SubmissionForScoreboard{
			State:     row.State,
			CreatedAt: row.CreatedAt,
		}
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
		return HandlePgErr(err)
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
		return HandlePgErr(err)
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
		return nil, HandlePgErr(err)
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
		return nil, HandlePgErr(err)
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
		return nil, HandlePgErr(err)
	}

	return r.GetContestJoinRequestByID(ctx, row.ID)
}

func (r *ContestsRepo) GetContestJoinRequestByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestJoinRequest, error) {
	row, err := r.queries.GetContestJoinRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, HandlePgErr(err)
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
		return nil, HandlePgErr(err)
	}

	return r.GetContestJoinRequestByID(ctx, row.ID)
}

func (r *ContestsRepo) ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]contestsmodel.ContestJoinRequest, error) {
	rows, err := r.queries.ListContestJoinRequests(ctx, sqlc.ListContestJoinRequestsParams{
		ContestID: contestID,
		Status:    status,
	})
	if err != nil {
		return nil, HandlePgErr(err)
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
		return nil, HandlePgErr(err)
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
		return HandlePgErr(err)
	}
	if affected != 1 {
		return pkg.Wrap(pkg.ErrConflict, nil, "request is no longer pending")
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

func mapDashboardContest(row sqlc.ListDashboardContestsRow) contestsmodel.DashboardContest {
	var lastSubTime *time.Time
	if row.LastSubTime != nil {
		if t, ok := row.LastSubTime.(time.Time); ok {
			lastSubTime = &t
		}
	}

	startTime := row.ContestStartTime

	endTime := row.ContestEndTime

	return contestsmodel.DashboardContest{
		ID:                 row.ContestID,
		Login:              row.ContestLogin,
		Title:              row.ContestTitle,
		StartTime:          startTime,
		EndTime:            endTime,
		CreatedAt:          row.ContestCreatedAt,
		OrganizationID:     row.OrgID,
		OrganizationName:   row.OrgName,
		OrganizationLogin:  row.OrgLogin,
		UserRole:           row.UserRole,
		LastSubmissionTime: lastSubTime,
	}
}
