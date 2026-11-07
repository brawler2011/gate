package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrganizationsRepo struct {
	q *sqlc.Queries
}

func NewOrganizationsRepo(db *pgxpool.Pool) *OrganizationsRepo {
	return &OrganizationsRepo{
		q: sqlc.New(db),
	}
}

func (r *OrganizationsRepo) CreateOrganization(ctx context.Context, input *organizationsmodel.CreateOrganizationInput) (*organizationsmodel.Organization, error) {
	joinPolicy := string(input.JoinPolicy)
	if joinPolicy == "" {
		joinPolicy = string(organizationsmodel.OrgJoinPolicyByRequest)
	}

	org, err := r.q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID:          uuid.New(),
		Login:       input.Login,
		Name:        input.Name,
		Description: input.Description,
		AvatarUrl:   input.AvatarURL,
		Column6:     joinPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", HandlePgErr(err))
	}

	return &organizationsmodel.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		JoinPolicy:  organizationsmodel.OrganizationJoinPolicy(org.JoinPolicy),
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.Organization, error) {
	org, err := r.q.GetOrganizationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
		}
		return nil, fmt.Errorf("failed to get organization by ID: %w", HandlePgErr(err))
	}

	return &organizationsmodel.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		JoinPolicy:  organizationsmodel.OrganizationJoinPolicy(org.JoinPolicy),
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) GetOrganizationByLogin(ctx context.Context, login string) (*organizationsmodel.Organization, error) {
	org, err := r.q.GetOrganizationByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization not found")
		}
		return nil, fmt.Errorf("failed to get organization by login: %w", HandlePgErr(err))
	}

	return &organizationsmodel.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		JoinPolicy:  organizationsmodel.OrganizationJoinPolicy(org.JoinPolicy),
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) ListOrganizations(ctx context.Context, filter *organizationsmodel.OrganizationFilter) ([]organizationsmodel.Organization, int32, error) {
	search := ""
	if filter.Search != "" {
		search = filter.Search
	}

	orgs, err := r.q.ListOrganizations(ctx, sqlc.ListOrganizationsParams{
		Column1: search,
		Limit:   filter.PageSize,
		Offset:  Offset(filter.Page, filter.PageSize),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list organizations: %w", HandlePgErr(err))
	}

	count, err := r.q.CountOrganizations(ctx, search)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count organizations: %w", HandlePgErr(err))
	}

	result := make([]organizationsmodel.Organization, len(orgs))
	for i, org := range orgs {
		result[i] = organizationsmodel.Organization{
			ID:          org.ID,
			Login:       org.Login,
			Name:        org.Name,
			Description: org.Description,
			AvatarURL:   org.AvatarUrl,
			JoinPolicy:  organizationsmodel.OrganizationJoinPolicy(org.JoinPolicy),
			CreatedAt:   org.CreatedAt,
			UpdatedAt:   org.UpdatedAt,
		}
	}

	return result, safeInt32(count), nil
}

func (r *OrganizationsRepo) UpdateOrganization(ctx context.Context, id uuid.UUID, input *organizationsmodel.UpdateOrganizationInput) error {
	params := sqlc.UpdateOrganizationParams{
		ID: id,
	}

	if input.Login != nil {
		params.Login = input.Login
	}
	if input.Name != nil {
		params.Name = input.Name
	}
	if input.Description != nil {
		params.Description = input.Description
	}
	if input.AvatarURL != nil {
		params.AvatarUrl = input.AvatarURL
	}
	if input.JoinPolicy != nil {
		jp := string(*input.JoinPolicy)
		params.JoinPolicy = &jp
	}

	if err := r.q.UpdateOrganization(ctx, params); err != nil {
		return fmt.Errorf("failed to update organization: %w", HandlePgErr(err))
	}

	return nil
}

func (r *OrganizationsRepo) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteOrganization(ctx, id); err != nil {
		return fmt.Errorf("failed to delete organization: %w", HandlePgErr(err))
	}
	return nil
}

func (r *OrganizationsRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error {
	if err := r.q.AddOrganizationMember(ctx, sqlc.AddOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}); err != nil {
		return fmt.Errorf("failed to add organization member: %w", HandlePgErr(err))
	}
	return nil
}

func (r *OrganizationsRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationMember, error) {
	member, err := r.q.GetOrganizationMember(ctx, sqlc.GetOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, pkg.Wrap(pkg.ErrNotFound, err, "organization member not found")
		}
		return nil, fmt.Errorf("failed to get organization member: %w", HandlePgErr(err))
	}

	return mapOrganizationMember(member), nil
}

func mapOrganizationMember(member sqlc.OrganizationMember) *organizationsmodel.OrganizationMember {
	return &organizationsmodel.OrganizationMember{
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           organizationsmodel.OrganizationRole(member.Role),
		CreatedAt:      member.CreatedAt,
	}
}

func (r *OrganizationsRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]organizationsmodel.OrganizationMember, error) {
	members, err := r.q.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization members: %w", HandlePgErr(err))
	}

	result := make([]organizationsmodel.OrganizationMember, len(members))
	for i, m := range members {
		var email string
		if m.Email != nil {
			email = *m.Email
		}
		result[i] = organizationsmodel.OrganizationMember{
			OrganizationID: m.OrganizationID,
			UserID:         m.UserID,
			Role:           organizationsmodel.OrganizationRole(m.Role),
			Username:       m.Username,
			Email:          email,
			CreatedAt:      m.CreatedAt,
		}
	}

	return result, nil
}

func (r *OrganizationsRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role organizationsmodel.OrganizationRole) error {
	if err := r.q.UpdateOrganizationMemberRole(ctx, sqlc.UpdateOrganizationMemberRoleParams{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}); err != nil {
		return fmt.Errorf("failed to update organization member role: %w", HandlePgErr(err))
	}
	return nil
}

func (r *OrganizationsRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	// Lock only the departing membership. Concurrent attachments hold FOR SHARE on
	// this row until commit, so cleanup sees all their committed dependent rows.
	if _, err := r.q.LockOrganizationMember(ctx, sqlc.LockOrganizationMemberParams{OrganizationID: orgID, UserID: userID}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return HandlePgErr(err)
	}
	if err := r.q.RemoveTeamMembersByOrgAndUser(ctx, sqlc.RemoveTeamMembersByOrgAndUserParams{OrganizationID: orgID, UserID: userID}); err != nil {
		return HandlePgErr(err)
	}
	if err := r.q.RemoveContestMembersByOrgAndUser(ctx, sqlc.RemoveContestMembersByOrgAndUserParams{OrganizationID: orgID, UserID: userID}); err != nil {
		return HandlePgErr(err)
	}
	if err := r.q.RemoveProblemMembersByOrgAndUser(ctx, sqlc.RemoveProblemMembersByOrgAndUserParams{OrganizationID: orgID, UserID: userID}); err != nil {
		return HandlePgErr(err)
	}

	if err := r.q.RemoveOrganizationMember(ctx, sqlc.RemoveOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
	}); err != nil {
		return fmt.Errorf("failed to remove organization member: %w", HandlePgErr(err))
	}
	return nil
}

func (r *OrganizationsRepo) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.Organization, error) {
	orgs, err := r.q.GetUserOrganizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organizations: %w", HandlePgErr(err))
	}

	result := make([]organizationsmodel.Organization, len(orgs))
	for i, org := range orgs {
		result[i] = organizationsmodel.Organization{
			ID:          org.ID,
			Login:       org.Login,
			Name:        org.Name,
			Description: org.Description,
			AvatarURL:   org.AvatarUrl,
			JoinPolicy:  organizationsmodel.OrganizationJoinPolicy(org.JoinPolicy),
			CreatedAt:   org.CreatedAt,
			UpdatedAt:   org.UpdatedAt,
		}
	}

	return result, nil
}

func (r *OrganizationsRepo) ResolveUserOrganizationID(ctx context.Context, userID uuid.UUID, requestedOrgID *uuid.UUID) (uuid.UUID, bool, error) {
	var orgID uuid.UUID
	var err error

	if requestedOrgID == nil {
		orgID, err = r.q.GetLatestUserOrganizationID(ctx, userID)
	} else {
		orgID, err = r.q.GetSpecificUserOrganizationID(ctx, sqlc.GetSpecificUserOrganizationIDParams{
			UserID:         userID,
			OrganizationID: *requestedOrgID,
		})
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("failed to resolve user organization: %w", HandlePgErr(err))
	}

	return orgID, true, nil
}

func (r *OrganizationsRepo) CreateInvitation(ctx context.Context, input *organizationsmodel.CreateOrganizationInvitationInput) (*organizationsmodel.OrganizationInvitation, error) {
	id := uuid.New()
	row, err := r.q.CreateOrganizationInvitation(ctx, sqlc.CreateOrganizationInvitationParams{
		ID:             id,
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
		InviterID:      input.InviterID,
		Role:           string(input.Role),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization invitation: %w", HandlePgErr(err))
	}

	return r.GetInvitationByID(ctx, row.ID)
}

func (r *OrganizationsRepo) GetInvitationByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationInvitation, error) {
	row, err := r.q.GetOrganizationInvitationByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get organization invitation: %w", HandlePgErr(err))
	}

	var email string
	if row.Email != nil {
		email = *row.Email
	}

	return &organizationsmodel.OrganizationInvitation{
		ID:                row.ID,
		OrganizationID:    row.OrganizationID,
		OrganizationName:  row.OrganizationName,
		OrganizationLogin: row.OrganizationLogin,
		UserID:            row.UserID,
		Username:          row.Username,
		Email:             email,
		InviterID:         row.InviterID,
		InviterUsername:   row.InviterUsername,
		Role:              organizationsmodel.OrganizationRole(row.Role),
		Status:            organizationsmodel.RequestStatus(row.Status),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) GetPendingInvitation(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationInvitation, error) {
	row, err := r.q.GetPendingOrganizationInvitation(ctx, sqlc.GetPendingOrganizationInvitationParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending organization invitation: %w", HandlePgErr(err))
	}

	return r.GetInvitationByID(ctx, row.ID)
}

func (r *OrganizationsRepo) ListInvitations(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error) {
	rows, err := r.q.ListOrganizationInvitations(ctx, sqlc.ListOrganizationInvitationsParams{
		OrganizationID: orgID,
		Status:         status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list organization invitations: %w", HandlePgErr(err))
	}

	invitations := make([]organizationsmodel.OrganizationInvitation, len(rows))
	for i, row := range rows {
		var email string
		if row.Email != nil {
			email = *row.Email
		}
		invitations[i] = organizationsmodel.OrganizationInvitation{
			ID:              row.ID,
			OrganizationID:  row.OrganizationID,
			UserID:          row.UserID,
			Username:        row.Username,
			Email:           email,
			InviterID:       row.InviterID,
			InviterUsername: row.InviterUsername,
			Role:            organizationsmodel.OrganizationRole(row.Role),
			Status:          organizationsmodel.RequestStatus(row.Status),
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
	}
	return invitations, nil
}

func (r *OrganizationsRepo) ListUserInvitations(ctx context.Context, userID uuid.UUID, status *string) ([]organizationsmodel.OrganizationInvitation, error) {
	rows, err := r.q.ListUserOrganizationInvitations(ctx, sqlc.ListUserOrganizationInvitationsParams{
		UserID: userID,
		Status: status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list user organization invitations: %w", HandlePgErr(err))
	}

	invitations := make([]organizationsmodel.OrganizationInvitation, len(rows))
	for i, row := range rows {
		invitations[i] = organizationsmodel.OrganizationInvitation{
			ID:                    row.ID,
			OrganizationID:        row.OrganizationID,
			OrganizationName:      row.OrganizationName,
			OrganizationLogin:     row.OrganizationLogin,
			OrganizationAvatarURL: row.OrganizationAvatarUrl,
			UserID:                row.UserID,
			InviterID:             row.InviterID,
			InviterUsername:       row.InviterUsername,
			Role:                  organizationsmodel.OrganizationRole(row.Role),
			Status:                organizationsmodel.RequestStatus(row.Status),
			CreatedAt:             row.CreatedAt,
			UpdatedAt:             row.UpdatedAt,
		}
	}
	return invitations, nil
}

func (r *OrganizationsRepo) UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus) error {
	affected, err := r.q.UpdateOrganizationInvitationStatus(ctx, sqlc.UpdateOrganizationInvitationStatusParams{
		ID:     id,
		Status: string(status),
	})
	if err != nil {
		return HandlePgErr(err)
	}
	if affected != 1 {
		return pkg.Wrap(pkg.ErrConflict, nil, "request is no longer pending")
	}
	return nil
}

func (r *OrganizationsRepo) CreateJoinRequest(ctx context.Context, input *organizationsmodel.CreateOrganizationJoinRequestInput) (*organizationsmodel.OrganizationJoinRequest, error) {
	id := uuid.New()
	row, err := r.q.CreateOrganizationJoinRequest(ctx, sqlc.CreateOrganizationJoinRequestParams{
		ID:             id,
		OrganizationID: input.OrganizationID,
		UserID:         input.UserID,
		Message:        input.Message,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization join request: %w", HandlePgErr(err))
	}

	return r.GetJoinRequestByID(ctx, row.ID)
}

func (r *OrganizationsRepo) GetJoinRequestByID(ctx context.Context, id uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error) {
	row, err := r.q.GetOrganizationJoinRequestByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkg.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get organization join request: %w", HandlePgErr(err))
	}

	var email string
	if row.Email != nil {
		email = *row.Email
	}

	reviewedBy := row.ReviewedBy

	return &organizationsmodel.OrganizationJoinRequest{
		ID:                row.ID,
		OrganizationID:    row.OrganizationID,
		OrganizationName:  row.OrganizationName,
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

func (r *OrganizationsRepo) GetPendingJoinRequest(ctx context.Context, orgID, userID uuid.UUID) (*organizationsmodel.OrganizationJoinRequest, error) {
	row, err := r.q.GetPendingOrganizationJoinRequest(ctx, sqlc.GetPendingOrganizationJoinRequestParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending organization join request: %w", HandlePgErr(err))
	}

	return r.GetJoinRequestByID(ctx, row.ID)
}

func (r *OrganizationsRepo) ListJoinRequests(ctx context.Context, orgID uuid.UUID, status *string) ([]organizationsmodel.OrganizationJoinRequest, error) {
	rows, err := r.q.ListOrganizationJoinRequests(ctx, sqlc.ListOrganizationJoinRequestsParams{
		OrganizationID: orgID,
		Status:         status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list organization join requests: %w", HandlePgErr(err))
	}

	requests := make([]organizationsmodel.OrganizationJoinRequest, len(rows))
	for i, row := range rows {
		var email string
		if row.Email != nil {
			email = *row.Email
		}
		reviewedBy := row.ReviewedBy
		requests[i] = organizationsmodel.OrganizationJoinRequest{
			ID:               row.ID,
			OrganizationID:   row.OrganizationID,
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

func (r *OrganizationsRepo) ListUserJoinRequests(ctx context.Context, userID uuid.UUID) ([]organizationsmodel.OrganizationJoinRequest, error) {
	rows, err := r.q.ListUserOrganizationJoinRequests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user organization join requests: %w", HandlePgErr(err))
	}

	requests := make([]organizationsmodel.OrganizationJoinRequest, len(rows))
	for i, row := range rows {
		requests[i] = organizationsmodel.OrganizationJoinRequest{
			ID:                row.ID,
			OrganizationID:    row.OrganizationID,
			OrganizationName:  row.OrganizationName,
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

func (r *OrganizationsRepo) UpdateJoinRequestStatus(ctx context.Context, id uuid.UUID, status organizationsmodel.RequestStatus, reviewedBy *uuid.UUID) error {
	pgReviewedBy := reviewedBy
	affected, err := r.q.UpdateOrganizationJoinRequestStatus(ctx, sqlc.UpdateOrganizationJoinRequestStatusParams{
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
