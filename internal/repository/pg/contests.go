package pg

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContestsRepo struct {
	queries *sqlc.Queries
}

func NewContestsRepo(db *pgxpool.Pool) *ContestsRepo {
	return &ContestsRepo{
		queries: sqlc.New(db),
	}
}

func (r *ContestsRepo) CreateContest(ctx context.Context, params *models.CreateContestParams) error {
	titlesJSON, err := marshalJSON(params.Titles)
	if err != nil {
		return err
	}
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
		Titles:         titlesJSON,
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
	var titlesJSON, settingsJSON, accessPolicyJSON []byte
	var err error

	if c.Titles != nil {
		titlesJSON, err = marshalJSON(*c.Titles)
		if err != nil {
			return err
		}
	}

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
		Titles:       titlesJSON,
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

	// For count, we'll just return the length of results since we don't have a dedicated count query
	// This is a limitation - ideally we'd have a CountUserAccessibleContests query
	count := int64(len(rows))

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error) {
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
	err := r.queries.AddContestProblem(ctx, sqlc.AddContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
		PackageID: c.PackageId,
		Ordinal:   0, // TODO: should be calculated or passed
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
	// TODO: Implement when contest_teams table is fully integrated
	// For now, return empty slice as teams feature is not fully implemented
	return []models.ContestTeam{}, nil
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

func mapContest(c sqlc.Contest) models.Contest {
	var titles map[string]string
	var settings map[string]interface{}
	var accessPolicy map[string]interface{}

	// Unmarshal JSON fields
	if len(c.Titles) > 0 {
		json.Unmarshal(c.Titles, &titles)
	}
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
		Titles:         titles,
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
	var titles map[string]string
	if len(c.Titles) > 0 {
		json.Unmarshal(c.Titles, &titles)
	}

	return models.ContestProblem{
		ContestID:  c.ContestID,
		ProblemID:  c.ProblemID,
		PackageID:  c.PackageID,
		Ordinal:    int(c.Ordinal),
		Titles:     titles,
		ShortName:  c.ShortName,
		Visibility: string(c.Visibility),
		CreatedAt:  c.CreatedAt,
	}
}

func mapListContestProblemsRow(c sqlc.ListContestProblemsRow) models.ContestProblem {
	var titles map[string]string
	if len(c.Titles) > 0 {
		json.Unmarshal(c.Titles, &titles)
	}

	return models.ContestProblem{
		ContestID:  c.ContestID,
		ProblemID:  c.ProblemID,
		PackageID:  c.PackageID,
		Ordinal:    int(c.Ordinal),
		Titles:     titles,
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
