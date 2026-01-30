package pg

import (
	"context"
	"fmt"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrganizationsRepo struct {
	db *pgxpool.Pool
	q  *sqlc.Queries
}

func NewOrganizationsRepo(db *pgxpool.Pool) interfaces.OrganizationsRepo {
	return &OrganizationsRepo{
		db: db,
		q:  sqlc.New(db),
	}
}

func (r *OrganizationsRepo) WithTx(tx pgx.Tx) interfaces.OrganizationsRepo {
	return &OrganizationsRepo{
		db: r.db,
		q:  r.q.WithTx(tx),
	}
}

func (r *OrganizationsRepo) CreateOrganization(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error) {
	org, err := r.q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID:          uuid.New(),
		Login:       input.Login,
		Name:        input.Name,
		Description: input.Description,
		AvatarUrl:   input.AvatarURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return &models.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	org, err := r.q.GetOrganizationByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by ID: %w", err)
	}

	return &models.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) GetOrganizationByLogin(ctx context.Context, login string) (*models.Organization, error) {
	org, err := r.q.GetOrganizationByLogin(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by login: %w", err)
	}

	return &models.Organization{
		ID:          org.ID,
		Login:       org.Login,
		Name:        org.Name,
		Description: org.Description,
		AvatarURL:   org.AvatarUrl,
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}, nil
}

func (r *OrganizationsRepo) ListOrganizations(ctx context.Context, filter *models.OrganizationFilter) ([]models.Organization, int32, error) {
	search := ""
	if filter.Search != "" {
		search = filter.Search
	}

	limit := int32(10)
	offset := int32(0)
	if filter.PageSize > 0 {
		limit = filter.PageSize
	}
	if filter.Page > 1 {
		offset = (filter.Page - 1) * limit
	}

	orgs, err := r.q.ListOrganizations(ctx, sqlc.ListOrganizationsParams{
		Column1: search,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list organizations: %w", err)
	}

	count, err := r.q.CountOrganizations(ctx, search)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count organizations: %w", err)
	}

	result := make([]models.Organization, len(orgs))
	for i, org := range orgs {
		result[i] = models.Organization{
			ID:          org.ID,
			Login:       org.Login,
			Name:        org.Name,
			Description: org.Description,
			AvatarURL:   org.AvatarUrl,
			CreatedAt:   org.CreatedAt,
			UpdatedAt:   org.UpdatedAt,
		}
	}

	return result, int32(count), nil
}

func (r *OrganizationsRepo) UpdateOrganization(ctx context.Context, id uuid.UUID, input *models.UpdateOrganizationInput) error {
	params := sqlc.UpdateOrganizationParams{
		ID: id,
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

	if err := r.q.UpdateOrganization(ctx, params); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	return nil
}

func (r *OrganizationsRepo) DeleteOrganization(ctx context.Context, id uuid.UUID) error {
	if err := r.q.DeleteOrganization(ctx, id); err != nil {
		return fmt.Errorf("failed to delete organization: %w", err)
	}
	return nil
}

func (r *OrganizationsRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole) error {
	if err := r.q.AddOrganizationMember(ctx, sqlc.AddOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           sqlc.OrganizationRole(role),
	}); err != nil {
		return fmt.Errorf("failed to add organization member: %w", err)
	}
	return nil
}

func (r *OrganizationsRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, error) {
	member, err := r.q.GetOrganizationMember(ctx, sqlc.GetOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get organization member: %w", err)
	}

	return &models.OrganizationMember{
		OrganizationID: member.OrganizationID,
		UserID:         member.UserID,
		Role:           models.OrganizationRole(member.Role),
		CreatedAt:      member.CreatedAt,
	}, nil
}

func (r *OrganizationsRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]models.OrganizationMember, error) {
	members, err := r.q.ListOrganizationMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization members: %w", err)
	}

	result := make([]models.OrganizationMember, len(members))
	for i, m := range members {
		result[i] = models.OrganizationMember{
			OrganizationID: m.OrganizationID,
			UserID:         m.UserID,
			Role:           models.OrganizationRole(m.Role),
			Username:       m.Username,
			Email:          m.Email,
			Name:           m.Name,
			Surname:        m.Surname,
			CreatedAt:      m.CreatedAt,
		}
	}

	return result, nil
}

func (r *OrganizationsRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.OrganizationRole) error {
	if err := r.q.UpdateOrganizationMemberRole(ctx, sqlc.UpdateOrganizationMemberRoleParams{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           sqlc.OrganizationRole(role),
	}); err != nil {
		return fmt.Errorf("failed to update organization member role: %w", err)
	}
	return nil
}

func (r *OrganizationsRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	if err := r.q.RemoveOrganizationMember(ctx, sqlc.RemoveOrganizationMemberParams{
		OrganizationID: orgID,
		UserID:         userID,
	}); err != nil {
		return fmt.Errorf("failed to remove organization member: %w", err)
	}
	return nil
}

func (r *OrganizationsRepo) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]models.Organization, error) {
	orgs, err := r.q.GetUserOrganizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user organizations: %w", err)
	}

	result := make([]models.Organization, len(orgs))
	for i, org := range orgs {
		result[i] = models.Organization{
			ID:          org.ID,
			Login:       org.Login,
			Name:        org.Name,
			Description: org.Description,
			AvatarURL:   org.AvatarUrl,
			CreatedAt:   org.CreatedAt,
			UpdatedAt:   org.UpdatedAt,
		}
	}

	return result, nil
}
