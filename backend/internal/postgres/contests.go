package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	queries *sqlc.Queries
}

func NewContestsRepo(db *pgxpool.Pool) *ContestsRepo {
	return &ContestsRepo{
		queries: sqlc.New(db),
	}
}

func (r *ContestsRepo) WithTx(tx pgx.Tx) *ContestsRepo {
	return &ContestsRepo{
		queries: sqlc.New(tx),
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
		OwnerID:        uuidPtrToNullUUID(params.OwnerID),
		Visibility:     params.Visibility,
		Title:          params.Title,
		Login:          params.Login,
		Description:    params.Description,
		Settings:       settingsJSON,
		StartTime:      timePtrToNullTime(params.StartTime),
		EndTime:        timePtrToNullTime(params.EndTime),
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
		Visibility:   stringToNullContestVisibility(c.Visibility),
		Settings:     settingsJSON,
		StartTime:    timePtrToNullTime(c.StartTime),
		StartTimeSet: c.StartTimeSet,
		EndTime:      timePtrToNullTime(c.EndTime),
		EndTimeSet:   c.EndTimeSet,
		OwnerID:      uuidPtrToNullUUID(c.OwnerID),
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

func (r *ContestsRepo) ListOrganizationContests(ctx context.Context, orgID uuid.UUID, search string, visibility string, page, pageSize int32) (*contestsmodel.ContestsList, error) {
	rows, err := r.queries.ListContests(ctx, sqlc.ListContestsParams{
		OrganizationID: orgID,
		Column2:        search,
		Column3:        visibility,
		Limit:          pageSize,
		Offset:         Offset(page, pageSize),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	count, err := r.queries.CountContests(ctx, sqlc.CountContestsParams{
		OrganizationID: orgID,
		Column2:        search,
		Column3:        visibility,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]contestsmodel.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListContestsRow(row)
	}

	return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(page, pageSize, safeInt32(count))}, nil
}

func (r *ContestsRepo) ListAdminContests(ctx context.Context, filter contestsmodel.AdminContestsFilter) (*contestsmodel.ContestsList, error) {
	var visStr string
	if filter.Visibility != nil {
		visStr = string(*filter.Visibility)
	}

	rows, err := r.queries.ListAllContests(ctx, sqlc.ListAllContestsParams{
		Column1: filter.Search,
		Column2: visStr,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	count, err := r.queries.CountAllContests(ctx, sqlc.CountAllContestsParams{
		Column1: filter.Search,
		Column2: visStr,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]contestsmodel.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListAllContestsRow(row)
	}

	return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(count))}, nil
}

func (r *ContestsRepo) ListUserContests(ctx context.Context, filter contestsmodel.UserContestsFilter) (*contestsmodel.ContestsList, error) {
	rows, err := r.queries.ListUserAccessibleContests(ctx, sqlc.ListUserAccessibleContestsParams{
		PUserID: filter.UserId,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	count := int64(len(rows))

	contests := make([]contestsmodel.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListUserAccessibleContestsRow(row)
	}

	return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(count))}, nil
}

func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter contestsmodel.WorkshopContestsFilter) (*contestsmodel.ContestsList, error) {
	if filter.OrganizationID != nil {
		rows, err := r.queries.ListUserAccessibleContestsByOrg(ctx, sqlc.ListUserAccessibleContestsByOrgParams{
			PUserID:        filter.UserId,
			OrganizationID: *filter.OrganizationID,
			Limit:          filter.PageSize,
			Offset:         Offset(filter.Page, filter.PageSize),
		})
		if err != nil {
			return nil, HandlePgErr(err)
		}
		contests := make([]contestsmodel.Contest, len(rows))
		for i, row := range rows {
			contests[i] = mapListUserAccessibleContestsByOrgRow(row)
		}
		return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(len(rows)))}, nil
	}

	rows, err := r.queries.ListUserAccessibleContests(ctx, sqlc.ListUserAccessibleContestsParams{
		PUserID: filter.UserId,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]contestsmodel.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListUserAccessibleContestsRow(row)
	}

	return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(len(rows)))}, nil
}

func (r *ContestsRepo) ListPublicContests(ctx context.Context, filter contestsmodel.PublicContestsFilter) (*contestsmodel.ContestsList, error) {
	rows, err := r.queries.ListAllContests(ctx, sqlc.ListAllContestsParams{
		Column1: filter.Search,
		Column2: string(contestsmodel.ContestVisibilityPublic),
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	count, err := r.queries.CountAllContests(ctx, sqlc.CountAllContestsParams{
		Column1: filter.Search,
		Column2: string(contestsmodel.ContestVisibilityPublic),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]contestsmodel.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListAllContestsRow(row)
	}

	return &contestsmodel.ContestsList{Contests: contests, Pagination: paging.NewPagination(filter.Page, filter.PageSize, safeInt32(count))}, nil
}

func (r *ContestsRepo) CreateContestProblem(ctx context.Context, c contestsmodel.ContestProblemCreation) error {
	packageId := c.PackageId
	if packageId == uuid.Nil {
		readyPkg, err := r.queries.GetReadyPackage(ctx, c.ProblemId)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return pkg.Wrap(pkg.ErrBadInput, nil, "problem has no ready package")
			}
			return HandlePgErr(err)
		}
		packageId = readyPkg.ID
	}

	problems, err := r.queries.ListContestProblems(ctx, c.ContestId)
	if err != nil {
		return HandlePgErr(err)
	}

	ordinal := safeInt32(len(problems) + 1)

	err = r.queries.AddContestProblem(ctx, sqlc.AddContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
		PackageID: packageId,
		Ordinal:   ordinal,
	})
	if err != nil {
		if errUpdate := r.queries.UpdateContestProblemPackage(ctx, sqlc.UpdateContestProblemPackageParams{
			ContestID: c.ContestId,
			ProblemID: c.ProblemId,
			PackageID: packageId,
		}); errUpdate == nil {
			return nil
		}
		return HandlePgErr(err)
	}
	return nil
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

	// Step 1: Temporarily assign large ordinals to avoid UNIQUE (contest_id, ordinal) constraint violations
	for i, p := range problems {
		tempOrdinal := int32(10000 + i)
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
	err := r.queries.UpdateContestProblemPackage(ctx, sqlc.UpdateContestProblemPackageParams{
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
	rows, err := r.queries.ListContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	// No pagination support in the query - return all members
	count := int64(len(rows))

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
	ownerID pgtype.UUID,
	visibility string,
	title string,
	login string,
	description string,
	settingsBytes []byte,
	startTime pgtype.Timestamptz,
	endTime pgtype.Timestamptz,
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
		OwnerID:           pgUUIDToUUIDPtr(ownerID),
		Visibility:        visibility,
		Title:             title,
		Login:             login,
		Description:       description,
		Settings:          settings,
		StartTime:         pgTimestamptzToTimePtr(startTime),
		EndTime:           pgTimestamptzToTimePtr(endTime),
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

func pgUUIDToUUIDPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	u := uuid.UUID(p.Bytes)
	return &u
}

func pgTimestamptzToTimePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func stringToNullContestVisibility(s *contestsmodel.ContestVisibility) sqlc.NullContestVisibility {
	if s == nil {
		return sqlc.NullContestVisibility{Valid: false}
	}
	return sqlc.NullContestVisibility{
		ContestVisibility: sqlc.ContestVisibility(*s),
		Valid:             true,
	}
}

func uuidPtrToNullUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{
		Bytes: *u,
		Valid: true,
	}
}

func timePtrToNullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
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
		UserID: userID,
		Limit:  limit,
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
	var firstACTime pgtype.Timestamptz
	if params.FirstACTime != nil {
		firstACTime = pgtype.Timestamptz{Time: *params.FirstACTime, Valid: true}
	}

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

	var firstAC *time.Time
	if row.FirstAcTime.Valid {
		firstAC = &row.FirstAcTime.Time
	}

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
		if row.ProblemID.Valid {
			probID := uuid.UUID(row.ProblemID.Bytes)
			var solved bool
			if row.Solved != nil {
				solved = *row.Solved
			}
			var failedAttempts int32
			if row.FailedAttempts != nil {
				failedAttempts = *row.FailedAttempts
			}
			var firstAC *time.Time
			if row.FirstAcTime.Valid {
				firstAC = &row.FirstAcTime.Time
			}

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

func (r *ContestsRepo) GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]contestsmodel.SubmissionForScoreboard, error) {
	rows, err := r.queries.GetSubmissionsForScoreboard(ctx, sqlc.GetSubmissionsForScoreboardParams{
		ContestID: nullableUUIDToPgtype(&contestID),
		OwnerID:   nullableUUIDToPgtype(&userID),
		ProblemID: nullableUUIDToPgtype(&problemID),
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
	var createdBy pgtype.UUID
	if params.CreatedBy != nil {
		createdBy = pgtype.UUID{Bytes: *params.CreatedBy, Valid: true}
	}

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

	var createdBy *uuid.UUID
	if row.CreatedBy.Valid {
		cb := uuid.UUID(row.CreatedBy.Bytes)
		createdBy = &cb
	}

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
		UserID:    nullableUUIDToPgtype(userID),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	blocks := make([]contestsmodel.ContestUserProblemBlock, len(rows))
	for i, row := range rows {
		var createdBy *uuid.UUID
		if row.CreatedBy.Valid {
			cb := uuid.UUID(row.CreatedBy.Bytes)
			createdBy = &cb
		}
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

	var reviewedBy *uuid.UUID
	if row.ReviewedBy.Valid {
		id := uuid.UUID(row.ReviewedBy.Bytes)
		reviewedBy = &id
	}

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
		var reviewedBy *uuid.UUID
		if row.ReviewedBy.Valid {
			id := uuid.UUID(row.ReviewedBy.Bytes)
			reviewedBy = &id
		}
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
	pgReviewedBy := pgtype.UUID{}
	if reviewedBy != nil {
		pgReviewedBy = pgtype.UUID{Bytes: *reviewedBy, Valid: true}
	}
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

func mapListContestsRow(c sqlc.ListContestsRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListAllContestsRow(c sqlc.ListAllContestsRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListUserAccessibleContestsRow(c sqlc.ListUserAccessibleContestsRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListUserAccessibleContestsByOrgRow(c sqlc.ListUserAccessibleContestsByOrgRow) contestsmodel.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
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

	var startTime *time.Time
	if row.ContestStartTime.Valid {
		startTime = &row.ContestStartTime.Time
	}

	var endTime *time.Time
	if row.ContestEndTime.Valid {
		endTime = &row.ContestEndTime.Time
	}

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

func (r *ContestsRepo) LockContest(ctx context.Context, id uuid.UUID) error {
	_, err := r.queries.LockContest(ctx, id)
	return HandlePgErr(err)
}
