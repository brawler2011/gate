package pg

import (
	"context"
	"time"

	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/gate149/core/pkg"
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
	_, err := r.queries.CreateContest(ctx, sqlc.CreateContestParams{
		ID:        params.Id,
		Title:     params.Title,
		CreatedBy: params.UserId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContest(ctx context.Context, id uuid.UUID) (models.Contest, error) {
	contest, err := r.queries.GetContest(ctx, id)
	if err != nil {
		return models.Contest{}, pkg.HandlePgErr(err)
	}
	return mapContest(contest), nil
}

func (r *ContestsRepo) UpdateContest(ctx context.Context, c models.ContestUpdateParams) error {
	err := r.queries.UpdateContest(ctx, sqlc.UpdateContestParams{
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

func (r *ContestsRepo) DeleteContest(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteContest(ctx, id)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListAdminContests(ctx, sqlc.ListAdminContestsParams{
		Limit:      filter.PageSize,
		Offset:     Offset(filter.Page, filter.PageSize),
		Search:     filter.Search,
		Visibility: contestVisibilityToStringPtr(filter.Visibility),
		SortBy:     stringToStringPtr(filter.SortBy),
		SortOrder:  sortOrderToStringPtr(filter.SortOrder),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountAdminContests(ctx, sqlc.CountAdminContestsParams{
		Search:     filter.Search,
		Visibility: contestVisibilityToStringPtr(filter.Visibility),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListUserContests(ctx, sqlc.ListUserContestsParams{
		CreatedBy: filter.UserId,
		Limit:     filter.PageSize,
		Offset:    Offset(filter.Page, filter.PageSize),
		Search:    stringToStringPtr(filter.Search),
		SortBy:    stringToStringPtr(filter.SortBy),
		SortOrder: sortOrderToStringPtr(filter.SortOrder),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountUserContests(ctx, sqlc.CountUserContestsParams{
		UserID: filter.UserId,
		Search: stringToStringPtr(filter.Search),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListWorkshopContests(ctx, sqlc.ListWorkshopContestsParams{
		UserID:    filter.UserId,
		Limit:     filter.PageSize,
		Offset:    Offset(filter.Page, filter.PageSize),
		Search:    stringToStringPtr(filter.Search),
		SortBy:    stringToStringPtr(filter.SortBy),
		SortOrder: sortOrderToStringPtr(filter.SortOrder),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountWorkshopContests(ctx, sqlc.CountWorkshopContestsParams{
		UserID: filter.UserId,
		Search: stringToStringPtr(filter.Search),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]models.Contest, int32, error) {
	rows, err := r.queries.ListPublicContests(ctx, sqlc.ListPublicContestsParams{
		Limit:     filter.PageSize,
		Offset:    Offset(filter.Page, filter.PageSize),
		Search:    filter.Search,
		SortBy:    stringToStringPtr(filter.SortBy),
		SortOrder: sortOrderToStringPtr(filter.SortOrder),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountPublicContests(ctx, filter.Search)
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	contests := make([]models.Contest, len(rows))
	for i, row := range rows {
		contests[i] = mapContest(row)
	}

	return contests, int32(count), nil
}

func (r *ContestsRepo) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	err := r.queries.CreateContestProblem(ctx, sqlc.CreateContestProblemParams{
		ProblemID: c.ProblemId,
		ContestID: c.ContestId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	err := r.queries.DeleteContestProblem(ctx, sqlc.DeleteContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (models.ContestProblem, error) {
	row, err := r.queries.GetContestProblem(ctx, sqlc.GetContestProblemParams{
		ContestID: c.ContestId,
		ProblemID: c.ProblemId,
	})
	if err != nil {
		return models.ContestProblem{}, pkg.HandlePgErr(err)
	}
	return mapGetContestProblemRow(row), nil
}

func (r *ContestsRepo) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]models.ContestProblem, error) {
	rows, err := r.queries.GetContestProblems(ctx, contestId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	problems := make([]models.ContestProblem, len(rows))
	for i, row := range rows {
		problems[i] = mapGetContestProblemsRow(row)
	}

	return problems, nil
}

func (r *ContestsRepo) ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]models.ContestMember, int32, error) {
	rows, err := r.queries.ListContestMembers(ctx, sqlc.ListContestMembersParams{
		ContestID: filter.ContestId,
		Limit:     filter.PageSize,
		Offset:    Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountContestMembers(ctx, filter.ContestId)
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	members := make([]models.ContestMember, len(rows))
	for i, row := range rows {
		members[i] = mapListContestMembersRow(row)
	}

	return members, int32(count), nil
}

func (r *ContestsRepo) CreateContestMember(ctx context.Context, c *models.CreateContestMemberParams) error {
	err := r.queries.CreateContestMember(ctx, sqlc.CreateContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
		Role:      c.Role,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (models.ContestMember, error) {
	row, err := r.queries.GetContestMember(ctx, sqlc.GetContestMemberParams{
		ContestID: c.ContestId,
		UserID:    c.UserId,
	})
	if err != nil {
		return models.ContestMember{}, pkg.HandlePgErr(err)
	}
	return mapGetContestMemberRow(row), nil
}

func (r *ContestsRepo) DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error {
	err := r.queries.DeleteContestMember(ctx, sqlc.DeleteContestMemberParams{
		UserID:    userId,
		ContestID: contestId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *ContestsRepo) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role models.ContestRole) error {
	err := r.queries.UpdateContestMember(ctx, sqlc.UpdateContestMemberParams{
		ContestID: contestId,
		UserID:    userId,
		Role:      role,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func mapContest(c sqlc.Contest) models.Contest {
	return models.Contest{
		ID:                     c.ID,
		Title:                  c.Title,
		Description:            c.Description,
		Visibility:             models.ContestRole(c.Visibility),
		MonitorScope:           models.ContestRole(c.MonitorScope),
		SubmissionsListScope:   models.ContestRole(c.SubmissionsListScope),
		SubmissionsReviewScope: models.ContestRole(c.SubmissionsReviewScope),
		CreatedBy:              pgUUIDToUUID(c.CreatedBy),
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}
}

func mapGetContestProblemRow(c sqlc.GetContestProblemRow) models.ContestProblem {
	return models.ContestProblem{
		ProblemID:        pgUUIDToUUID(c.ProblemID),
		Title:            nullStringToString(c.Title),
		TimeLimit:        nullInt32ToInt32(c.TimeLimit),
		MemoryLimit:      nullInt32ToInt32(c.MemoryLimit),
		Position:         c.Position,
		LegendHtml:       nullStringToString(c.LegendHtml),
		InputFormatHtml:  nullStringToString(c.InputFormatHtml),
		OutputFormatHtml: nullStringToString(c.OutputFormatHtml),
		NotesHtml:        nullStringToString(c.NotesHtml),
		ScoringHtml:      nullStringToString(c.ScoringHtml),
		CreatedAt:        nullTimeToTimeVal(c.CreatedAt),
		UpdatedAt:        nullTimeToTimeVal(c.UpdatedAt),
	}
}

func mapGetContestProblemsRow(c sqlc.GetContestProblemsRow) models.ContestProblem {
	return models.ContestProblem{
		ProblemID:   pgUUIDToUUID(c.ProblemID),
		Title:       nullStringToString(c.Title),
		TimeLimit:   nullInt32ToInt32(c.TimeLimit),
		MemoryLimit: nullInt32ToInt32(c.MemoryLimit),
		Position:    c.Position,
		CreatedAt:   nullTimeToTimeVal(c.CreatedAt),
		UpdatedAt:   nullTimeToTimeVal(c.UpdatedAt),
	}
}

func mapListContestMembersRow(c sqlc.ListContestMembersRow) models.ContestMember {
	return models.ContestMember{
		UserID:      pgUUIDToUUID(c.UserID),
		ContestID:   c.ContestID,
		Username:    nullStringToString(c.Username),
		Role:        models.UserRole(c.Role.UserRole),
		ContestRole: models.ContestRole(c.ContestRole),
		KratosID:    pgUUIDToUUID(c.KratosID).String(),
		CreatedAt:   nullTimeToTimeVal(c.CreatedAt),
		UpdatedAt:   nullTimeToTimeVal(c.UpdatedAt),
	}
}

func mapGetContestMemberRow(c sqlc.GetContestMemberRow) models.ContestMember {
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
