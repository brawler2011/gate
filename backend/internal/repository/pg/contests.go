package pg

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func (r *ContestsRepo) CreateContest(ctx context.Context, params *models.CreateContestParams) error {
	settingsJSON, err := marshalJSON(params.Settings)
	if err != nil {
		return err
	}
	accessPolicyJSON, err := marshalJSON(params.AccessPolicy)
	if err != nil {
		return err
	}

	_, err = r.queries.CreateContest(ctx, sqlc.CreateContestParams{
		ID:             params.ID,
		OrganizationID: params.OrganizationID,
		OwnerID:        uuidPtrToNullUUID(params.OwnerID),
		Visibility:     params.Visibility,
		Title:          params.Title,
		ShortName:      params.ShortName,
		Description:    params.Description,
		Settings:       settingsJSON,
		AccessPolicy:   accessPolicyJSON,
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
	return mapContest(contest), nil
}

func (r *ContestsRepo) UpdateContest(ctx context.Context, c models.ContestUpdateParams) error {
	var settingsJSON, accessPolicyJSON []byte
	var err error

	if c.Settings != nil {
		settingsJSON, err = marshalJSON(*c.Settings)
		if err != nil {
			return err
		}
	}

	if c.AccessPolicy != nil {
		accessPolicyJSON, err = marshalJSON(*c.AccessPolicy)
		if err != nil {
			return err
		}
	}

	err = r.queries.UpdateContest(ctx, sqlc.UpdateContestParams{
		ID:           c.ID,
		Title:        c.Title,
		Description:  c.Description,
		Visibility:   stringToNullContestVisibility(c.Visibility),
		Settings:     settingsJSON,
		AccessPolicy: accessPolicyJSON,
		StartTime:    timePtrToNullTime(c.StartTime),
		EndTime:      timePtrToNullTime(c.EndTime),
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

func (r *ContestsRepo) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]models.Contest, int32, error) {
	visStr := ""
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
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
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
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error) {
	var rows []sqlc.Contest
	var err error

	if filter.OrganizationID != nil {
		rows, err = r.queries.ListUserAccessibleContestsByOrg(ctx, sqlc.ListUserAccessibleContestsByOrgParams{
			PUserID:        filter.UserId,
			OrganizationID: *filter.OrganizationID,
			Limit:          filter.PageSize,
			Offset:         Offset(filter.Page, filter.PageSize),
		})
	} else {
		rows, err = r.queries.ListUserAccessibleContests(ctx, sqlc.ListUserAccessibleContestsParams{
			PUserID: filter.UserId,
			Limit:   filter.PageSize,
			Offset:  Offset(filter.Page, filter.PageSize),
		})
	}
	if err != nil {
		return nil, 0, HandlePgErr(err)
	}

	count := int64(len(rows))

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
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
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
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

	ordinal := int32(len(problems) + 1)

	err = r.queries.AddContestProblem(ctx, sqlc.AddContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
		PackageID: packageId,
		Ordinal:   ordinal,
	})
	if err != nil {
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

	if err := r.syncContestTeamPermissionsMask(ctx, contestID, teamID, role); err != nil {
		return err
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

	if err := r.syncContestTeamPermissionsMask(ctx, contestID, teamID, role); err != nil {
		return err
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

	return members, int32(count), nil
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

	if err := r.syncContestMemberPermissionsMask(ctx, c.ContestId, c.UserId, c.Role); err != nil {
		return err
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

	if err := r.syncContestMemberPermissionsMask(ctx, contestId, userId, role); err != nil {
		return err
	}

	return nil
}

func (r *ContestsRepo) syncContestMemberPermissionsMask(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
	role models.ContestRole,
) error {
	mask, ok := models.ContestRoleDefaultPermissionMask(role)
	if !ok {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid contest role for permissions mask")
	}

	_, err := r.db.Exec(
		ctx,
		"UPDATE contest_members SET permissions_mask = $3 WHERE contest_id = $1 AND user_id = $2",
		contestID,
		userID,
		int64(mask),
	)
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *ContestsRepo) syncContestTeamPermissionsMask(
	ctx context.Context,
	contestID uuid.UUID,
	teamID uuid.UUID,
	role models.ContestRole,
) error {
	mask, ok := models.ContestRoleDefaultPermissionMask(role)
	if !ok {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid contest role for team permissions mask")
	}

	_, err := r.db.Exec(
		ctx,
		"UPDATE contest_teams SET permissions_mask = $3 WHERE contest_id = $1 AND team_id = $2",
		contestID,
		teamID,
		int64(mask),
	)
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func mapContest(c sqlc.Contest) models.Contest {
	var settings map[string]interface{}
	var accessPolicy map[string]interface{}

	// Unmarshal JSON fields
	if len(c.Settings) > 0 {
		json.Unmarshal(c.Settings, &settings)
	}
	if len(c.AccessPolicy) > 0 {
		json.Unmarshal(c.AccessPolicy, &accessPolicy)
	}

	return models.Contest{
		ID:             c.ID,
		OrganizationID: c.OrganizationID,
		OwnerID:        pgUUIDToUUIDPtr(c.OwnerID),
		Visibility:     c.Visibility,
		Title:          c.Title,
		ShortName:      c.ShortName,
		Description:    c.Description,
		Settings:       settings,
		AccessPolicy:   accessPolicy,
		StartTime:      pgTimestamptzToTimePtr(c.StartTime),
		EndTime:        pgTimestamptzToTimePtr(c.EndTime),
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
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
	mask := models.ContestPermissionMask(c.PermissionsMask)
	return models.ContestMember{
		UserID:          c.UserID,
		ContestID:       c.ContestID,
		ContestRole:     models.ContestRole(c.Role),
		PermissionsMask: &mask,
	}
}

func mapContestTeam(c sqlc.ListContestTeamsRow) models.ContestTeam {
	mask := models.ContestPermissionMask(c.PermissionsMask)
	return models.ContestTeam{
		ContestID:       c.ContestID,
		TeamID:          c.TeamID,
		Role:            c.Role,
		PermissionsMask: &mask,
		TeamName:        c.TeamName,
		TeamSlug:        c.TeamSlug,
		CreatedAt:       c.CreatedAt,
	}
}

func pgUUIDToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return p.Bytes
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

func nullTimeToTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func nullTimeToTimeVal(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func nullStringToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullInt32ToInt32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
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

func stringToNullContestRole(s *models.ContestRole) sqlc.NullContestRole {
	if s == nil {
		return sqlc.NullContestRole{Valid: false}
	}
	return sqlc.NullContestRole{
		ContestRole: sqlc.ContestRole(*s),
		Valid:       true,
	}
}

func timeToNullTime(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{
		Time:  *t,
		Valid: true,
	}
}

func contestVisibilityToStringPtr(v *models.ContestVisibility) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func stringToStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sortOrderToStringPtr(s models.SortOrder) *string {
	if s == "" {
		return nil
	}
	str := string(s)
	return &str
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
	return json.Marshal(v)
}
