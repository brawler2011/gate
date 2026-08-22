package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
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
	db      *pgxpool.Pool
}

func NewContestsRepo(db *pgxpool.Pool) *ContestsRepo {
	return &ContestsRepo{
		queries: sqlc.New(db),
		db:      db,
	}
}

func (r *ContestsRepo) WithTx(tx pgx.Tx) interfaces.ContestsRepo {
	return &ContestsRepo{
		queries: sqlc.New(tx),
		db:      r.db,
	}
}

func (r *ContestsRepo) CreateContest(ctx context.Context, params *models.CreateContestParams) error {
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

func (r *ContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	contest, err := r.queries.GetContestByID(ctx, id)
	if err != nil {
		return models.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByIDRow(contest), nil
}

func (r *ContestsRepo) GetContestByLogin(ctx context.Context, orgID uuid.UUID, login string) (models.Contest, error) {
	contest, err := r.queries.GetContestByLogin(ctx, sqlc.GetContestByLoginParams{
		OrganizationID: orgID,
		Lower:          login,
	})
	if err != nil {
		return models.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByLoginRow(contest), nil
}

func (r *ContestsRepo) GetContestByOrgLoginAndContestLogin(ctx context.Context, orgLogin, contestLogin string) (models.Contest, error) {
	contest, err := r.queries.GetContestByOrgLoginAndContestLogin(ctx, sqlc.GetContestByOrgLoginAndContestLoginParams{
		Lower:   orgLogin,
		Lower_2: contestLogin,
	})
	if err != nil {
		return models.Contest{}, HandlePgErr(err)
	}
	return mapGetContestByOrgLoginAndContestLoginRow(contest), nil
}

func (r *ContestsRepo) UpdateContest(ctx context.Context, c models.ContestUpdateParams) error {
	var settingsJSON []byte
	var err error

	if c.Settings != nil {
		settingsJSON, err = marshalJSON(*c.Settings)
		if err != nil {
			return err
		}
	}

	err = r.queries.UpdateContest(ctx, sqlc.UpdateContestParams{
		ID:          c.ID,
		Login:       c.Login,
		Title:       c.Title,
		Description: c.Description,
		Visibility:  stringToNullContestVisibility(c.Visibility),
		Settings:    settingsJSON,
		StartTime:   timePtrToNullTime(c.StartTime),
		EndTime:     timePtrToNullTime(c.EndTime),
		OwnerID:     uuidPtrToNullUUID(c.OwnerID),
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

func (r *ContestsRepo) ListOrganizationContests(ctx context.Context, orgID uuid.UUID, search string, visibility string, page, pageSize int32) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListContests(ctx, sqlc.ListContestsParams{
		OrganizationID: orgID,
		Column2:        search,
		Column3:        visibility,
		Limit:          pageSize,
		Offset:         Offset(page, pageSize),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	count, err := r.queries.CountContests(ctx, sqlc.CountContestsParams{
		OrganizationID: orgID,
		Column2:        search,
		Column3:        visibility,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListContestsRow(row)
	}

	return contests, safeInt32(count), nil
}

func (r *ContestsRepo) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]models.Contest, int32, error) {
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
		return nil, 0, HandlePgErr(err)
	}

	count, err := r.queries.CountAllContests(ctx, sqlc.CountAllContestsParams{
		Column1: filter.Search,
		Column2: visStr,
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListAllContestsRow(row)
	}

	return contests, safeInt32(count), nil
}

func (r *ContestsRepo) ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListUserAccessibleContests(ctx, sqlc.ListUserAccessibleContestsParams{
		PUserID: filter.UserId,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	count := int64(len(rows))

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListUserAccessibleContestsRow(row)
	}

	return contests, safeInt32(count), nil
}

func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error) {
	if filter.OrganizationID != nil {
		rows, err := r.queries.ListUserAccessibleContestsByOrg(ctx, sqlc.ListUserAccessibleContestsByOrgParams{
			PUserID:        filter.UserId,
			OrganizationID: *filter.OrganizationID,
			Limit:          filter.PageSize,
			Offset:         Offset(filter.Page, filter.PageSize),
		})
		if err != nil {
			return nil, 0, HandlePgErr(err)
		}
		contests := make([]models.Contest, len(rows))
		for i, row := range rows {
			contests[i] = mapListUserAccessibleContestsByOrgRow(row)
		}
		return contests, safeInt32(len(rows)), nil
	}

	rows, err := r.queries.ListUserAccessibleContests(ctx, sqlc.ListUserAccessibleContestsParams{
		PUserID: filter.UserId,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListUserAccessibleContestsRow(row)
	}

	return contests, safeInt32(len(rows)), nil
}

func (r *ContestsRepo) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListAllContests(ctx, sqlc.ListAllContestsParams{
		Column1: filter.Search,
		Column2: string(models.ContestVisibilityPublic),
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	count, err := r.queries.CountAllContests(ctx, sqlc.CountAllContestsParams{
		Column1: filter.Search,
		Column2: string(models.ContestVisibilityPublic),
	})
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapListAllContestsRow(row)
	}

	return contests, safeInt32(count), nil
}

func (r *ContestsRepo) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
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

func (r *ContestsRepo) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	err := r.queries.RemoveContestProblem(ctx, sqlc.RemoveContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) ReorderContestProblems(ctx context.Context, contestId uuid.UUID, problems []models.ContestProblemReorderItem) error {
	var q *sqlc.Queries
	var commit func() error

	if r.db != nil {
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return HandlePgErr(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		q = r.queries.WithTx(tx)
		commit = func() error { return tx.Commit(ctx) }
	} else {
		q = r.queries
		commit = func() error { return nil }
	}

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

	return commit()
}

func (r *ContestsRepo) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error) {
	row, err := r.queries.GetContestProblem(ctx, sqlc.GetContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return models.ContestProblem{}, HandlePgErr(err)
	}
	return mapGetContestProblemRow(row), nil
}

func (r *ContestsRepo) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error) {
	rows, err := r.queries.ListContestProblems(ctx, contestId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	problems := make([]models.ContestProblem, len(rows))
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

func (r *ContestsRepo) GetContestTeams(ctx context.Context, contestId uuid.UUID) ([]models.ContestTeam, error) {
	rows, err := r.queries.ListContestTeams(ctx, contestId)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	teams := make([]models.ContestTeam, len(rows))
	for i, row := range rows {
		teams[i] = mapContestTeam(row)
	}

	return teams, nil
}

func (r *ContestsRepo) CreateContestTeam(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
	role models.ContestRole,
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
	role models.ContestRole,
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

func (r *ContestsRepo) ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]models.ContestMember, int32, error) {
	rows, err := r.queries.ListContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	// No pagination support in the query - return all members
	count := int64(len(rows))

	members := make([]models.ContestMember, len(rows))
	for i, row := range rows {
		members[i] = mapListContestMembersRow(row)
	}

	return members, safeInt32(count), nil
}

func (r *ContestsRepo) CreateContestMember(ctx context.Context, c *models.CreateContestMemberParams) error {
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

func (r *ContestsRepo) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	member, err := r.queries.GetContestMember(ctx, sqlc.GetContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
	})
	if err != nil {
		return models.ContestMember{}, HandlePgErr(err)
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

func (r *ContestsRepo) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role models.ContestRole) error {
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

func (r *ContestsRepo) ListUserContestMemberships(ctx context.Context, userID uuid.UUID) ([]models.UserContestMembership, error) {
	rows, err := r.queries.ListUserContestMemberships(ctx, userID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	memberships := make([]models.UserContestMembership, len(rows))
	for i, row := range rows {
		memberships[i] = models.UserContestMembership{
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
) models.Contest {
	var settings map[string]interface{}

	if len(settingsBytes) > 0 {
		_ = json.Unmarshal(settingsBytes, &settings)
	}

	return models.Contest{
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

func mapGetContestByIDRow(c sqlc.GetContestByIDRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestByLoginRow(c sqlc.GetContestByLoginRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestByOrgLoginAndContestLoginRow(c sqlc.GetContestByOrgLoginAndContestLoginRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListContestsRow(c sqlc.ListContestsRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListAllContestsRow(c sqlc.ListAllContestsRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListUserAccessibleContestsRow(c sqlc.ListUserAccessibleContestsRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapListUserAccessibleContestsByOrgRow(c sqlc.ListUserAccessibleContestsByOrgRow) models.Contest {
	return mapContestFields(c.ID, c.OrganizationID, c.OrgLogin, c.OwnerID, string(c.Visibility), c.Title, c.Login, c.Description, c.Settings, c.StartTime, c.EndTime, c.CreatedAt, c.UpdatedAt)
}

func mapGetContestProblemRow(c sqlc.GetContestProblemRow) models.ContestProblem {
	return models.ContestProblem{
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

func mapListContestProblemsRow(c sqlc.ListContestProblemsRow) models.ContestProblem {
	return models.ContestProblem{
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

func mapListContestMembersRow(c sqlc.ListContestMembersRow) models.ContestMember {
	return models.ContestMember{
		UserID:      c.UserID,
		ContestID:   c.ContestID,
		Username:    c.Username,
		ContestRole: c.Role,
		CreatedAt:   c.CreatedAt,
	}
}

func mapContestMember(c sqlc.ContestMember) models.ContestMember {
	return models.ContestMember{
		UserID:      c.UserID,
		ContestID:   c.ContestID,
		ContestRole: models.ContestRole(c.Role),
	}
}

func mapContestTeam(c sqlc.ListContestTeamsRow) models.ContestTeam {
	return models.ContestTeam{
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

func stringToNullContestVisibility(s *models.ContestVisibility) sqlc.NullContestVisibility {
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

func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

func (r *ContestsRepo) ListDashboardContests(ctx context.Context, userID uuid.UUID, limit int32) ([]models.DashboardContest, error) {
	rows, err := r.queries.ListDashboardContests(ctx, sqlc.ListDashboardContestsParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	contests := make([]models.DashboardContest, len(rows))
	for i, row := range rows {
		contests[i] = mapDashboardContest(row)
	}

	return contests, nil
}

func mapDashboardContest(row sqlc.ListDashboardContestsRow) models.DashboardContest {
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

	return models.DashboardContest{
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

func (r *ContestsRepo) UpsertContestProblemResult(ctx context.Context, params *models.UpsertContestProblemResultParams) error {
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

func (r *ContestsRepo) GetContestProblemResult(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestProblemResult, error) {
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

	return &models.ContestProblemResult{
		ContestID:      row.ContestID,
		UserID:         row.UserID,
		ProblemID:      row.ProblemID,
		Solved:         row.Solved,
		FailedAttempts: row.FailedAttempts,
		FirstACTime:    firstAC,
		TimeMinutes:    row.TimeMinutes,
	}, nil
}

func (r *ContestsRepo) GetContestScoreboardFromStandings(ctx context.Context, contestID uuid.UUID) ([]models.ContestProblemResult, map[uuid.UUID]string, error) {
	rows, err := r.queries.GetContestScoreboardFromStandings(ctx, contestID)
	if err != nil {
		return nil, nil, HandlePgErr(err)
	}

	userMap := make(map[uuid.UUID]string)
	results := make([]models.ContestProblemResult, 0, len(rows))

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

			results = append(results, models.ContestProblemResult{
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

func (r *ContestsRepo) GetSubmissionsForScoreboard(ctx context.Context, contestID, userID, problemID uuid.UUID) ([]models.SubmissionForScoreboard, error) {
	rows, err := r.queries.GetSubmissionsForScoreboard(ctx, sqlc.GetSubmissionsForScoreboardParams{
		ContestID: nullableUUIDToPgtype(&contestID),
		OwnerID:   nullableUUIDToPgtype(&userID),
		ProblemID: nullableUUIDToPgtype(&problemID),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	subs := make([]models.SubmissionForScoreboard, len(rows))
	for i, row := range rows {
		subs[i] = models.SubmissionForScoreboard{
			State:     row.State,
			CreatedAt: row.CreatedAt,
		}
	}
	return subs, nil
}

func (r *ContestsRepo) CreateContestUserProblemBlock(ctx context.Context, params *models.CreateContestUserProblemBlockParams) error {
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

func (r *ContestsRepo) GetContestUserProblemBlock(ctx context.Context, contestID, userID, problemID uuid.UUID) (*models.ContestUserProblemBlock, error) {
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

	return &models.ContestUserProblemBlock{
		ContestID: row.ContestID,
		UserID:    row.UserID,
		ProblemID: row.ProblemID,
		Reason:    row.Reason,
		CreatedBy: createdBy,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *ContestsRepo) ListContestUserProblemBlocks(ctx context.Context, contestID uuid.UUID, userID *uuid.UUID) ([]models.ContestUserProblemBlock, error) {
	rows, err := r.queries.ListContestUserProblemBlocks(ctx, sqlc.ListContestUserProblemBlocksParams{
		ContestID: contestID,
		UserID:    nullableUUIDToPgtype(userID),
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	blocks := make([]models.ContestUserProblemBlock, len(rows))
	for i, row := range rows {
		var createdBy *uuid.UUID
		if row.CreatedBy.Valid {
			cb := uuid.UUID(row.CreatedBy.Bytes)
			createdBy = &cb
		}
		blocks[i] = models.ContestUserProblemBlock{
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

func (r *ContestsRepo) CreateContestJoinRequest(ctx context.Context, input *models.CreateContestJoinRequestInput) (*models.ContestJoinRequest, error) {
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

func (r *ContestsRepo) GetContestJoinRequestByID(ctx context.Context, id uuid.UUID) (*models.ContestJoinRequest, error) {
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

	return &models.ContestJoinRequest{
		ID:                row.ID,
		ContestID:         row.ContestID,
		ContestTitle:      row.ContestTitle,
		ContestLogin:      row.ContestLogin,
		OrganizationLogin: row.OrganizationLogin,
		UserID:            row.UserID,
		Username:          row.Username,
		Email:             email,
		Message:           row.Message,
		Status:            models.RequestStatus(row.Status),
		ReviewedBy:        reviewedBy,
		ReviewerUsername:  row.ReviewerUsername,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (r *ContestsRepo) GetPendingContestJoinRequest(ctx context.Context, contestID, userID uuid.UUID) (*models.ContestJoinRequest, error) {
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

func (r *ContestsRepo) ListContestJoinRequests(ctx context.Context, contestID uuid.UUID, status *string) ([]models.ContestJoinRequest, error) {
	rows, err := r.queries.ListContestJoinRequests(ctx, sqlc.ListContestJoinRequestsParams{
		ContestID: contestID,
		Status:    status,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	requests := make([]models.ContestJoinRequest, len(rows))
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
		requests[i] = models.ContestJoinRequest{
			ID:               row.ID,
			ContestID:        row.ContestID,
			UserID:           row.UserID,
			Username:         row.Username,
			Email:            email,
			Message:          row.Message,
			Status:           models.RequestStatus(row.Status),
			ReviewedBy:       reviewedBy,
			ReviewerUsername: row.ReviewerUsername,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		}
	}
	return requests, nil
}

func (r *ContestsRepo) ListUserContestJoinRequests(ctx context.Context, userID uuid.UUID) ([]models.ContestJoinRequest, error) {
	rows, err := r.queries.ListUserContestJoinRequests(ctx, userID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	requests := make([]models.ContestJoinRequest, len(rows))
	for i, row := range rows {
		requests[i] = models.ContestJoinRequest{
			ID:                row.ID,
			ContestID:         row.ContestID,
			ContestTitle:      row.ContestTitle,
			ContestLogin:      row.ContestLogin,
			OrganizationLogin: row.OrganizationLogin,
			UserID:            row.UserID,
			Message:           row.Message,
			Status:            models.RequestStatus(row.Status),
			CreatedAt:         row.CreatedAt,
			UpdatedAt:         row.UpdatedAt,
		}
	}
	return requests, nil
}

func (r *ContestsRepo) UpdateContestJoinRequestStatus(ctx context.Context, id uuid.UUID, status models.RequestStatus, reviewedBy *uuid.UUID) error {
	pgReviewedBy := pgtype.UUID{}
	if reviewedBy != nil {
		pgReviewedBy = pgtype.UUID{Bytes: *reviewedBy, Valid: true}
	}
	err := r.queries.UpdateContestJoinRequestStatus(ctx, sqlc.UpdateContestJoinRequestStatusParams{
		ID:         id,
		Status:     string(status),
		ReviewedBy: pgReviewedBy,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}




