package contests

import (
	"context"
	"time"

	contestssqlc "github.com/gate149/core/internal/contests/sqlc"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *Repository) GetContest(ctx context.Context, id uuid.UUID) (contestssqlc.GetContestRow, error) {
	contest, err := r.queries.GetContest(ctx, id)
	if err != nil {
		return contestssqlc.GetContestRow{}, pkg.HandlePgErr(err)
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
		StartTime:              timeToNullTime(c.StartTime),
		EndTime:                timeToNullTime(c.EndTime),
		ScoringMode:            stringToNullContestScoringMode(c.ScoringMode),
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

func (r *Repository) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]contestssqlc.ListAdminContestsRow, int64, error) {
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

func (r *Repository) ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]contestssqlc.ListUserContestsRow, int64, error) {
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

func (r *Repository) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]contestssqlc.ListWorkshopContestsRow, int64, error) {
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

func (r *Repository) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]contestssqlc.ListPublicContestsRow, int64, error) {
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

// Access Requests

func (r *Repository) CreateAccessRequest(ctx context.Context, id uuid.UUID, contestId uuid.UUID, userId uuid.UUID, status string) error {
	_, err := r.queries.CreateAccessRequest(ctx, contestssqlc.CreateAccessRequestParams{
		ID:        id,
		ContestID: contestId,
		UserID:    userId,
		Status:    contestssqlc.ContestAccessRequestStatus(status),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetAccessRequest(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) (contestssqlc.ContestAccessRequest, error) {
	row, err := r.queries.GetAccessRequest(ctx, contestssqlc.GetAccessRequestParams{
		ContestID: contestId,
		UserID:    userId,
	})
	if err != nil {
		return contestssqlc.ContestAccessRequest{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) ListAccessRequests(ctx context.Context, contestId uuid.UUID, status *string, limit int64, offset int64) ([]contestssqlc.ListAccessRequestsRow, int64, error) {
	var statusParam *string
	if status != nil {
		statusParam = status
	}

	rows, err := r.queries.ListAccessRequests(ctx, contestssqlc.ListAccessRequestsParams{
		ContestID: contestId,
		Status:    statusParam,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountAccessRequests(ctx, contestssqlc.CountAccessRequestsParams{
		ContestID: contestId,
		Status:    statusParam,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, count, nil
}

func (r *Repository) UpdateAccessRequestStatus(ctx context.Context, id uuid.UUID, status string) error {
	err := r.queries.UpdateAccessRequestStatus(ctx, contestssqlc.UpdateAccessRequestStatusParams{
		ID:     id,
		Status: contestssqlc.ContestAccessRequestStatus(status),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// Invitations

func (r *Repository) CreateInvitation(ctx context.Context, id uuid.UUID, contestId uuid.UUID, userId uuid.UUID, invitedBy uuid.UUID, status string) error {
	_, err := r.queries.CreateInvitation(ctx, contestssqlc.CreateInvitationParams{
		ID:        id,
		ContestID: contestId,
		UserID:    userId,
		InvitedBy: invitedBy,
		Status:    contestssqlc.ContestInvitationStatus(status),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetInvitation(ctx context.Context, id uuid.UUID) (contestssqlc.ContestInvitation, error) {
	row, err := r.queries.GetInvitation(ctx, id)
	if err != nil {
		return contestssqlc.ContestInvitation{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) GetInvitationByUser(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) (contestssqlc.ContestInvitation, error) {
	row, err := r.queries.GetInvitationByUser(ctx, contestssqlc.GetInvitationByUserParams{
		ContestID: contestId,
		UserID:    userId,
	})
	if err != nil {
		return contestssqlc.ContestInvitation{}, pkg.HandlePgErr(err)
	}
	return row, nil
}

func (r *Repository) ListInvitations(ctx context.Context, contestId uuid.UUID, status *string, limit int64, offset int64) ([]contestssqlc.ListInvitationsRow, int64, error) {
	var statusParam *string
	if status != nil {
		statusParam = status
	}

	rows, err := r.queries.ListInvitations(ctx, contestssqlc.ListInvitationsParams{
		ContestID: contestId,
		Status:    statusParam,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountInvitations(ctx, contestssqlc.CountInvitationsParams{
		ContestID: contestId,
		Status:    statusParam,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, count, nil
}

func (r *Repository) ListUserInvitations(ctx context.Context, userId uuid.UUID, limit int64, offset int64) ([]contestssqlc.ListUserInvitationsRow, int64, error) {
	rows, err := r.queries.ListUserInvitations(ctx, contestssqlc.ListUserInvitationsParams{
		UserID: userId,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	// For user invitations, we don't have a dedicated count query, so we return len(rows) as count
	// In a production environment, you'd want to add a CountUserInvitations query
	return rows, int64(len(rows)), nil
}

func (r *Repository) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error {
	err := r.queries.UpdateInvitationStatus(ctx, contestssqlc.UpdateInvitationStatusParams{
		ID:     id,
		Status: contestssqlc.ContestInvitationStatus(status),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

// Problem Position Management

func (r *Repository) UpdateContestProblemPosition(ctx context.Context, contestId uuid.UUID, problemId uuid.UUID, position int64) error {
	err := r.queries.UpdateContestProblemPosition(ctx, contestssqlc.UpdateContestProblemPositionParams{
		ContestID: contestId,
		ProblemID: problemId,
		Position:  int32(position),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) ReorderProblemsAfterDelete(ctx context.Context, contestId uuid.UUID, deletedPosition int64) error {
	err := r.queries.ReorderContestProblemsAfterDelete(ctx, contestssqlc.ReorderContestProblemsAfterDeleteParams{
		ContestID:       contestId,
		DeletedPosition: int32(deletedPosition),
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetMaxProblemPosition(ctx context.Context, contestId uuid.UUID) (int64, error) {
	maxPos, err := r.queries.GetMaxProblemPosition(ctx, contestId)
	if err != nil {
		return -1, pkg.HandlePgErr(err)
	}
	
	// maxPos might be int32 or int64 depending on database driver
	switch v := maxPos.(type) {
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	default:
		return -1, nil
	}
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

func stringToNullContestScoringMode(s *string) contestssqlc.NullContestScoringMode {
	if s == nil {
		return contestssqlc.NullContestScoringMode{Valid: false}
	}
	return contestssqlc.NullContestScoringMode{
		ContestScoringMode: contestssqlc.ContestScoringMode(*s),
		Valid:              true,
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

// Monitor

func (r *Repository) GetContestMonitor(ctx context.Context, contestId uuid.UUID, limit int64, offset int64) ([]contestssqlc.GetContestMonitorRow, int64, error) {
	rows, err := r.queries.GetContestMonitor(ctx, contestssqlc.GetContestMonitorParams{
		ContestID: contestId,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	count, err := r.queries.CountContestMonitorRows(ctx, contestId)
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return rows, count, nil
}

func (r *Repository) GetMonitorProblemDetails(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) ([]contestssqlc.GetMonitorProblemDetailsRow, error) {
	rows, err := r.queries.GetMonitorProblemDetails(ctx, contestssqlc.GetMonitorProblemDetailsParams{
		ContestID: contestId,
		UserID:    userId,
	})
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return rows, nil
}
